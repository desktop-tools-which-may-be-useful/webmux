import { expect, test } from "vitest";
import {
  domText,
  host,
  installTerminalFixture,
  observeGlyphPixels,
  openTerminal,
} from "./terminal-fixture";

installTerminalFixture();

// One-pixel RGBA images whose colour identifies the screen they belong to.
const red = "/wAA/w==";
const blue = "AAD//w==";

function image(id: number, pixels: string) {
  return `\x1b_Ga=T,f=32,s=1,v=1,i=${id},c=2,r=1,q=2;${pixels}\x1b\\`;
}

// The colours of the images a viewer can currently see.
function visibleImages() {
  return Array.from(
    host().querySelectorAll<HTMLCanvasElement>(
      ".xterm-graphics-layer .xterm-graphics-image",
    ),
  )
    .filter((canvas) => canvas.style.display !== "none" && canvas.isConnected)
    .map((canvas) => {
      const [r, , b] = canvas.getContext("2d")!.getImageData(0, 0, 1, 1).data;
      return r! > b! ? "red" : "blue";
    })
    .sort();
}

// Samples what each painted frame showed: after a frame's callbacks, before
// later tasks change the DOM again.
function sampleFrames() {
  const frames: { text: string; images: string[] }[] = [];
  let running = true;
  const next = () =>
    requestAnimationFrame(() =>
      setTimeout(() => {
        if (!running) return;
        frames.push({ text: domText(), images: visibleImages() });
        next();
      }, 0),
    );
  next();
  return {
    frames,
    stop() {
      running = false;
    },
  };
}

async function settle() {
  for (let i = 0; i < 6; i++)
    await new Promise((resolve) => requestAnimationFrame(resolve));
}

// Every frame must pair the old text with the old image only, and the new
// text with the new image: never a cleared image layer under old text, a
// new image over old text, or new text whose image has not appeared yet.
function expectConsistentFrames(
  frames: { text: string; images: string[] }[],
  oldText: string,
  newText: string,
) {
  for (const frame of frames) {
    if (frame.text.includes(oldText))
      expect(frame).toMatchObject({ images: ["red"] });
    if (frame.text.includes(newText))
      expect(frame).toMatchObject({ images: ["blue"] });
  }
  expect(frames.at(-1)).toMatchObject({ images: ["blue"] });
  expect(frames.at(-1)!.text).toContain(newText);
}

test("a synchronized redraw swaps text and kitty images in the same frame", async () => {
  observeGlyphPixels(true);
  const { socket } = await openTerminal();
  socket.receive(0, `OLD-SCREEN\r\n${image(1, red)}\r\n`);
  await expect.poll(visibleImages).toEqual(["red"]);
  const sampler = sampleFrames();
  // The omp pattern: clear, delete every image, reprint text and images.
  socket.receive(
    0,
    "\x1b[?2026h\x1b[2J\x1b[3J\x1b[H\x1b_Ga=d,d=A,q=2\x1b\\NEW-SCREEN\r\n",
  );
  await settle();
  socket.receive(0, `${image(2, blue)}\r\nmore new text\r\n`);
  await settle();
  expect(domText()).toContain("OLD-SCREEN");
  expect(visibleImages()).toEqual(["red"]);
  socket.receive(0, "\x1b[?2026l");
  await expect.poll(visibleImages).toEqual(["blue"]);
  await settle();
  sampler.stop();
  expectConsistentFrames(sampler.frames, "OLD-SCREEN", "NEW-SCREEN");
});

test("a synchronized redraw that scrolls keeps the previous screen until it ends", async () => {
  // xterm's DOM renderer repaints all rows for the selection refresh that
  // every scroll queues; that repaint must wait for the update to end too.
  observeGlyphPixels(true);
  const { socket } = await openTerminal();
  socket.receive(0, "OLD-SCREEN\r\n");
  await expect.poll(domText).toContain("OLD-SCREEN");
  const lines = Array.from({ length: 80 }, (_, i) => `new line ${i}`);
  socket.receive(0, `\x1b[?2026h\x1b[2J\x1b[H${lines.join("\r\n")}`);
  await settle();
  expect(domText()).toContain("OLD-SCREEN");
  socket.receive(0, "\x1b[?2026l");
  await expect.poll(domText).toContain("new line 79");
  expect(domText()).not.toContain("OLD-SCREEN");
});

test("restoring a snapshot keeps the old screen and images until the replay is painted", async () => {
  observeGlyphPixels(true);
  const { socket } = await openTerminal();
  socket.receive(0, `OLD-SCREEN\r\n${image(1, red)}\r\n`);
  await expect.poll(visibleImages).toEqual(["red"]);
  // Every DOM state the replay passes through, painted or not: reset() and
  // the image reset must never leave blank rows or an empty image layer.
  const states: { text: string; images: string[] }[] = [];
  const observer = new MutationObserver(() =>
    states.push({ text: domText().trim(), images: visibleImages() }),
  );
  observer.observe(host(), {
    subtree: true,
    childList: true,
    attributes: true,
    characterData: true,
  });
  const sampler = sampleFrames();
  // History long enough that xterm parses it across several frames.
  const history = Array.from({ length: 40000 }, (_, i) => `history ${i}`);
  socket.receive(
    1,
    `${history.join("\r\n")}\r\nNEW-SCREEN\r\n${image(2, blue)}\r\nreplayed tail\r\n`,
  );
  await expect.poll(visibleImages).toEqual(["blue"]);
  await expect.poll(domText).toContain("replayed tail");
  await settle();
  sampler.stop();
  observer.disconnect();
  expectConsistentFrames(sampler.frames, "OLD-SCREEN", "NEW-SCREEN");
  for (const state of states) {
    expect(state.text).not.toBe("");
    expect(state.images).not.toEqual([]);
    if (state.text.includes("OLD-SCREEN"))
      expect(state).toMatchObject({ images: ["red"] });
  }
});

// xterm ends a synchronized update on its own after one second. A check made
// later would see that timeout, not the path under test, so it is skipped.
const syncTimeoutMargin = 800;

async function frames(count: number) {
  for (let i = 0; i < count; i++)
    await new Promise((resolve) => requestAnimationFrame(resolve));
}

async function startSynchronizedUpdate() {
  observeGlyphPixels(true);
  const { socket } = await openTerminal();
  socket.receive(0, "OLD-SCREEN\r\n");
  await expect.poll(domText).toContain("OLD-SCREEN");
  socket.receive(0, "\x1b[?2026h\x1b[2J\x1b[HNEW-SCREEN\r\n");
  return { socket, started: performance.now() };
}

test("refocusing during a synchronized update keeps the previous screen", async (context) => {
  // The DOM renderer repaints the cursor row itself on focus and blur.
  const { socket, started } = await startSynchronizedUpdate();
  const input = host().querySelector<HTMLTextAreaElement>(
    ".xterm-helper-textarea",
  )!;
  input.focus();
  await frames(2);
  input.blur();
  await frames(2);
  const shown = domText();
  if (performance.now() - started > syncTimeoutMargin)
    context.skip("the runner reached xterm's synchronized-output timeout");
  expect(shown).toContain("OLD-SCREEN");
  socket.receive(0, "\x1b[?2026l");
  await expect.poll(domText).toContain("NEW-SCREEN");
});

test("resizing during a synchronized update keeps the previous screen", async (context) => {
  // A grid resize makes the DOM renderer rebuild and repaint its rows itself.
  const { socket, started } = await startSynchronizedUpdate();
  const rows = () => host().querySelectorAll(".xterm-rows > div").length;
  const before = rows();
  host().style.height = "500px";
  await expect.poll(rows, { interval: 10 }).not.toBe(before);
  await frames(2);
  const shown = domText();
  if (performance.now() - started > syncTimeoutMargin)
    context.skip("the runner reached xterm's synchronized-output timeout");
  expect(shown).toContain("OLD-SCREEN");
  socket.receive(0, "\x1b[?2026l");
  await expect.poll(domText).toContain("NEW-SCREEN");
});

test("a snapshot that stops inside an image upload ends synchronization without corrupting it", async () => {
  // A raw snapshot may stop inside an unfinished sequence; the live bytes
  // that follow complete it. No terminator may be written in between.
  observeGlyphPixels(true);
  const { socket } = await openTerminal();
  socket.receive(0, "OLD-SCREEN\r\n");
  await expect.poll(domText).toContain("OLD-SCREEN");
  socket.receive(1, "NEW-SCREEN\r\n\x1b_Ga=T,f=32,s=1,v=1,i=3,c=2,r=1,q=2;");
  // Well before xterm's one second synchronized-output timeout.
  await expect.poll(domText, { timeout: 500 }).toContain("NEW-SCREEN");
  socket.receive(0, `${blue}\x1b\\after image\r\n`);
  await expect.poll(visibleImages).toEqual(["blue"]);
  await expect.poll(domText).toContain("after image");
});
