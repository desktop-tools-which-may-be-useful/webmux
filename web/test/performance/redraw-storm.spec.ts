import { fileURLToPath } from "node:url";
import { type Page } from "@playwright/test";
import { test, expect, createShell, runCommand, quote } from "../e2e/fixtures";
import { observePerformance } from "./performance";

// A resize makes an omp-like TUI reprint its whole transcript, text and Kitty
// images interleaved, as hundreds of small PTY writes. Measure what reaches the
// browser and how many visible repaints one such storm costs at 4x CPU.
const program = fileURLToPath(
  new URL("../fixtures/redraw-storm-demo.mjs", import.meta.url),
);

type Storm = {
  outputMessages: number;
  outputBytes: number;
  textFrames: number;
  graphicsFrames: number;
  visualFrames: number;
  durationMs: number;
  maxFrameGapMs: number;
  longTaskMs: number;
  fontLoads: number;
};

declare global {
  interface Window {
    stormProbe: {
      recording: boolean;
      messages: { time: number; bytes: number }[];
      start(): void;
      stop(): Promise<Omit<Storm, "longTaskMs"> & { start: number }>;
    };
  }
}

async function installProbe(page: Page) {
  await page.addInitScript(() => {
    const messages: { time: number; bytes: number }[] = [];
    const NativeWebSocket = window.WebSocket;
    window.WebSocket = class extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        super(url, protocols);
        if (!new URL(url, location.href).pathname.startsWith("/ws/s/")) return;
        this.addEventListener("message", (event: MessageEvent) => {
          if (!probe.recording || !(event.data instanceof ArrayBuffer)) return;
          if (new Uint8Array(event.data)[0] !== 0) return;
          messages.push({
            time: performance.now(),
            bytes: event.data.byteLength - 1,
          });
        });
      }
    };
    // Mutations only mark a frame for inspection; a frame counts as a repaint
    // when what a viewer sees changed: the rows' text or the visible images
    // and their positions (hidden canvases staged for later do not count).
    let observers: MutationObserver[] = [];
    let text = false;
    let graphics = false;
    let shownText = "";
    let shownImages = "";
    const textNow = () =>
      document.querySelector(".xterm-rows")?.textContent ?? "";
    const imagesNow = () =>
      Array.from(
        document.querySelectorAll<HTMLCanvasElement>(
          ".xterm-graphics-layer canvas",
        ),
      )
        .filter(
          (canvas) => canvas.isConnected && canvas.style.display !== "none",
        )
        .map((canvas) => `${canvas.style.transform}/${canvas.width}`)
        .join();
    let frames: { time: number; text: boolean; graphics: boolean }[] = [];
    let start = 0;
    let fontLoads = 0;
    document.fonts.addEventListener("loadingdone", () => {
      if (probe.recording) fontLoads++;
    });
    const sample = (time: number) => {
      if (!probe.recording) return;
      const nextText = text ? textNow() : shownText;
      const nextImages = graphics ? imagesNow() : shownImages;
      frames.push({
        time,
        text: nextText !== shownText,
        graphics: nextImages !== shownImages,
      });
      shownText = nextText;
      shownImages = nextImages;
      text = false;
      graphics = false;
      requestAnimationFrame(sample);
    };
    const probe: Window["stormProbe"] = {
      recording: false,
      messages,
      start() {
        messages.length = 0;
        fontLoads = 0;
        frames = [];
        text = false;
        graphics = false;
        shownText = textNow();
        shownImages = imagesNow();
        const rows = document.querySelector(".xterm-rows");
        const layer = document.querySelector(".xterm-graphics-layer");
        const textObserver = new MutationObserver(() => (text = true));
        const graphicsObserver = new MutationObserver(() => (graphics = true));
        const options = {
          subtree: true,
          childList: true,
          characterData: true,
          attributes: true,
        };
        if (rows) textObserver.observe(rows, options);
        if (layer) graphicsObserver.observe(layer, options);
        observers = [textObserver, graphicsObserver];
        start = performance.now();
        probe.recording = true;
        requestAnimationFrame(sample);
      },
      async stop() {
        // Let the final mutation records land in one more frame.
        await new Promise((resolve) => requestAnimationFrame(resolve));
        probe.recording = false;
        for (const observer of observers) observer.disconnect();
        const changed = frames.filter((frame) => frame.text || frame.graphics);
        const last = changed.at(-1)?.time ?? start;
        let gap = 0;
        for (let i = 1; i < frames.length; i++) {
          if (frames[i]!.time > last) break;
          gap = Math.max(gap, frames[i]!.time - frames[i - 1]!.time);
        }
        return {
          start,
          outputMessages: messages.length,
          outputBytes: messages.reduce((sum, m) => sum + m.bytes, 0),
          textFrames: frames.filter((frame) => frame.text).length,
          graphicsFrames: frames.filter((frame) => frame.graphics).length,
          visualFrames: changed.length,
          durationMs: Math.round(last - start),
          maxFrameGapMs: Math.round(gap),
          fontLoads,
        };
      },
    };
    window.stormProbe = probe;
  });
}

function rows(page: Page) {
  return page
    .locator(".xterm-rows > div")
    .evaluateAll((items) =>
      items.map((row) => (row.textContent ?? "").replaceAll(" ", " ")),
    );
}

async function stormMarker(page: Page) {
  const marker = /storm (\d+) (\d+)x(\d+) done/;
  for (const row of (await rows(page)).reverse()) {
    const match = marker.exec(row);
    if (match) return Number(match[1]);
  }
  return 0;
}

// Wait until the marker of a newer storm is on screen and nothing has been
// repainted for a while, so late image decodes are counted too.
async function settle(page: Page, after: number) {
  await expect
    .poll(() => stormMarker(page), { timeout: 30_000 })
    .toBeGreaterThan(after);
  let quiet = 0;
  let snapshot = "";
  while (quiet < 3) {
    await page.waitForTimeout(250);
    const next = await page.evaluate(
      () =>
        (document.querySelector(".xterm-rows")?.textContent ?? "") +
        (document.querySelector(".xterm-graphics-layer")?.innerHTML ?? ""),
    );
    quiet = next === snapshot ? quiet + 1 : 0;
    snapshot = next;
  }
  return stormMarker(page);
}

function median(values: number[]) {
  const sorted = values.toSorted((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)]!;
}

for (const sync of [true, false]) {
  test(`4x CPU: mixed text and image redraw storm on resize (${sync ? "synchronized" : "unsynchronized"} output)`, async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name !== "chromium-desktop",
      "Viewport resizing drives the PTY size on desktop.",
    );
    test.setTimeout(180_000);
    await observePerformance(page);
    await installProbe(page);
    await createShell(page, `Redraw storm ${sync ? "sync" : "nosync"}`);
    await runCommand(
      page,
      `${quote(process.execPath)} ${quote(program)}${sync ? "" : " --no-sync"}`,
    );
    let generation = await settle(page, 0);
    const longTasks = () =>
      page.evaluate(() =>
        window.webmuxPerformance.longTasks.reduce((a, b) => a + b, 0),
      );
    const storms: Storm[] = [];
    for (const width of [1100, 1280, 1000, 1280, 1150, 1280]) {
      const tasksBefore = await longTasks();
      await page.evaluate(() => window.stormProbe.start());
      await page.setViewportSize({ width, height: 800 });
      generation = await settle(page, generation);
      const { start: _, ...storm } = await page.evaluate(() =>
        window.stormProbe.stop(),
      );
      storms.push({
        ...storm,
        longTaskMs: Math.round((await longTasks()) - tasksBefore),
      });
    }
    const summary = Object.fromEntries(
      (Object.keys(storms[0]!) as (keyof Storm)[]).map((key) => [
        key,
        median(storms.map((storm) => storm[key])),
      ]),
    );
    console.log(
      `redraw storm (${sync ? "sync" : "nosync"}) median:`,
      JSON.stringify(summary),
      "\n  per storm (messages/visualFrames/ms/maxGap/fontLoads):",
      storms
        .map((s) =>
          [
            s.outputMessages,
            s.visualFrames,
            s.durationMs,
            s.maxFrameGapMs,
            s.fontLoads,
          ].join("/"),
        )
        .join(" "),
    );
    await testInfo.attach("redraw-storm.json", {
      body: JSON.stringify({ sync, summary, storms }, null, 2),
      contentType: "application/json",
    });
    await page.screenshot({ path: testInfo.outputPath("redraw-storm.png") });
  });
}
