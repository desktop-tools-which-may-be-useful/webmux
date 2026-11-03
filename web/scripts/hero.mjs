// Composes the README hero image docs/images/hero.jpg from the screenshots that
// `pnpm --dir web screenshots` already produced: the desktop omp session in a
// browser window and the same conversation in the phone chat view.
//
//   pnpm --dir web screenshots:hero
//
// It only reads committed screenshots, so it needs no backend or agent. Set
// PLAYWRIGHT_CHROMIUM_EXECUTABLE to use a preinstalled Chromium.
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { chromium } from "@playwright/test";

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const images = join(webRoot, "..", "docs", "images");
const asset = (name) => pathToFileURL(join(images, name)).href;

const page = `<!doctype html>
<meta charset="utf-8">
<style>
  * { box-sizing: border-box; margin: 0; }
  body {
    width: 1800px; height: 1040px; overflow: hidden; position: relative;
    font-family: "Inter", "Noto Sans CJK SC", "PingFang SC", sans-serif;
    background:
      radial-gradient(900px 600px at 18% 10%, #1d4d34 0%, transparent 60%),
      radial-gradient(800px 600px at 95% 100%, #14402b 0%, transparent 60%),
      #0b1210;
  }
  .browser {
    position: absolute; left: 70px; top: 80px; width: 1360px;
    border-radius: 18px; overflow: hidden; background: #111814;
    border: 1px solid #2a3a32;
    box-shadow: 0 40px 120px rgba(0, 0, 0, .55);
  }
  .bar {
    height: 44px; display: flex; align-items: center; gap: 9px;
    padding: 0 18px; background: #18211c; border-bottom: 1px solid #2a3a32;
  }
  .dot { width: 13px; height: 13px; border-radius: 50%; }
  .url {
    margin-left: 18px; padding: 5px 16px; border-radius: 8px;
    background: #0f1512; color: #8fa397; font-size: 15px;
  }
  .browser img { display: block; width: 100%; }
  .phone {
    position: absolute; right: 90px; bottom: 56px; width: 352px;
    padding: 12px; border-radius: 52px; background: #050807;
    border: 2px solid #2f3d36;
    box-shadow: 0 40px 100px rgba(0, 0, 0, .7);
  }
  .phone img { display: block; width: 100%; border-radius: 40px; }
  .tag {
    position: absolute; padding: 10px 20px; border-radius: 999px;
    background: #7ee2a8; color: #07130c; font-size: 22px; font-weight: 700;
    box-shadow: 0 12px 30px rgba(0, 0, 0, .4);
  }
</style>
<div class="browser">
  <div class="bar">
    <span class="dot" style="background:#ff5f57"></span>
    <span class="dot" style="background:#febc2e"></span>
    <span class="dot" style="background:#28c840"></span>
    <span class="url">webmux · omp · bookshelf-api</span>
  </div>
  <img src="${asset("desktop-omp.png")}">
</div>
<div class="phone"><img src="${asset("mobile-chat.png")}"></div>
<span class="tag" style="left: 110px; top: 30px">💻 电脑上用 TUI 开工</span>
<span class="tag" style="right: 120px; top: 230px">📱 手机上切到聊天接着聊</span>
`;

// A file:// page may load the file:// screenshots; about:blank may not.
const scratch = mkdtempSync(join(tmpdir(), "webmux-hero-"));
const html = join(scratch, "hero.html");
writeFileSync(html, page);

const browser = await chromium.launch({
  executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE || undefined,
});
try {
  const tab = await browser.newPage({
    viewport: { width: 1800, height: 1040 },
    deviceScaleFactor: 1,
  });
  await tab.goto(pathToFileURL(html).href, { waitUntil: "load" });
  await tab.evaluate(() => document.fonts.ready);
  const broken = await tab.evaluate(
    () => [...document.images].filter((img) => img.naturalWidth === 0).length,
  );
  if (broken > 0) throw new Error(`${broken} screenshot(s) failed to load`);
  const out = join(images, "hero.jpg");
  await tab.screenshot({ path: out, type: "jpeg", quality: 88 });
  console.log(`wrote ${out}`);
} finally {
  await browser.close();
  rmSync(scratch, { recursive: true, force: true });
}
