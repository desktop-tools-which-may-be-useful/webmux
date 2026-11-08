import type { Page } from "@playwright/test";
import { test, expect, createShell } from "./fixtures";

// Controls whose label must stay on one line. Mark an element that may
// legitimately wrap with `data-allow-wrap`.
const SINGLE_LINE =
  "button, [role=tab], [role=menuitem], [role=option], th, [data-slot=badge], nav a";

type Wrapped = { desc: string; lines: number; text: string; width: number };

// Counts the visual lines each matching control's text occupies, from the
// client rects of its text nodes, and reports controls that span more than one.
async function findWrapped(page: Page): Promise<Wrapped[]> {
  return page.evaluate((selector) => {
    const found: Wrapped[] = [];
    for (const element of document.querySelectorAll<HTMLElement>(selector)) {
      if (element.closest("[data-allow-wrap]")) continue;
      const box = element.getBoundingClientRect();
      if (box.width === 0 || box.height === 0) continue;
      const style = getComputedStyle(element);
      if (style.visibility === "hidden" || style.display === "none") continue;
      const tops: number[] = [];
      const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        if (!node.textContent?.trim()) continue;
        const range = document.createRange();
        range.selectNodeContents(node);
        for (const rect of range.getClientRects()) {
          if (rect.width === 0) continue;
          if (!tops.some((top) => Math.abs(top - rect.top) < rect.height / 2))
            tops.push(rect.top);
        }
      }
      if (tops.length < 2) continue;
      const classes = [...element.classList].slice(0, 3).join(".");
      found.push({
        desc: `${element.tagName.toLowerCase()}${classes ? "." + classes : ""}`,
        lines: tops.length,
        text: (element.textContent ?? "")
          .trim()
          .replace(/\s+/g, " ")
          .slice(0, 40),
        width: Math.round(box.width),
      });
    }
    return found;
  }, SINGLE_LINE);
}

async function expectNoWrap(page: Page, label: string) {
  const wrapped = await findWrapped(page);
  expect
    .soft(
      wrapped,
      `${label}: ${wrapped.length} control(s) wrap onto several lines`,
    )
    .toEqual([]);
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - innerWidth,
  );
  expect
    .soft(overflow, `${label}: horizontal page overflow`)
    .toBeLessThanOrEqual(0);
}

for (const path of [
  "/",
  "/new",
  "/settings",
  "/settings/tunnel",
  "/settings/tailscale",
]) {
  test(`controls stay on one line at the phone width: ${path}`, async ({
    page,
  }) => {
    await page.goto(path);
    await expect(page.getByRole("main")).toBeVisible();
    await page.waitForLoadState("networkidle");
    await expectNoWrap(page, path);
  });
}

test("controls stay on one line in a session", async ({ page }) => {
  await createShell(page);
  await expectNoWrap(page, "session");
});
