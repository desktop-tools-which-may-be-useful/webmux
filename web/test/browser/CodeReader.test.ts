import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./CodeReaderHarness.svelte";

const shadow = () =>
  document.querySelector<HTMLElement>("[data-code-reader] diffs-container")
    ?.shadowRoot;
const row = (line: number) =>
  shadow()?.querySelector<HTMLElement>(`[data-line="${line}"]`);
const gutter = (line: number) =>
  shadow()?.querySelector<HTMLElement>(`[data-column-number="${line}"]`);
const code = () => shadow()?.querySelector<HTMLElement>("[data-code]");
const scrollbar = () =>
  document.querySelector<HTMLElement>("[data-code-scrollbar]");
const token = (text: string) =>
  [...(shadow()?.querySelectorAll<HTMLElement>("[data-line] span") ?? [])].find(
    (element) => element.textContent?.trim() === text,
  );
const tokenColor = (text: string) => {
  const element = token(text);
  return element ? getComputedStyle(element).color : "";
};
afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  vi.unstubAllGlobals();
});

// Toggles the coarse-pointer query and notifies the reader's listener.
function coarsePointer() {
  const listeners = new Set<() => void>();
  let matches = true;
  const query = {
    media: "(pointer: coarse)",
    get matches() {
      return matches;
    },
    addEventListener(type: string, callback: (() => void) | null) {
      if (type === "change" && callback) listeners.add(callback);
    },
    removeEventListener(type: string, callback: (() => void) | null) {
      if (type === "change" && callback) listeners.delete(callback);
    },
  };
  const real = window.matchMedia.bind(window);
  vi.stubGlobal("matchMedia", (value: string) =>
    value === "(pointer: coarse)" ? query : real(value),
  );
  return (fine: boolean) => {
    matches = !fine;
    for (const listener of listeners) listener();
  };
}

test("keeps the selected line and its reference when the pointer becomes fine", async () => {
  const setCoarse = coarsePointer();
  const harness = await render(Harness, { text: "one\ntwo\nthree" });
  await expect.poll(() => shadow()?.textContent ?? "").toContain("three");
  await page.elementLocator(gutter(2)!).click();
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(/2-2/);
  expect(harness.component.capture().selection).toEqual({ start: 2, end: 2 });
  setCoarse(false);
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(/2-2/);
  expect(harness.component.capture().selection).toEqual({ start: 2, end: 2 });
});

test("read-only virtualized full file, real line selection, search and goto", async () => {
  const text = Array.from(
    { length: 1000 },
    (_, i) => `const line${i + 1} = ${i + 1};`,
  ).join("\n");
  render(Harness, { text });
  await expect
    .poll(() => shadow()?.textContent ?? "")
    .toContain("const line1 = 1;");
  expect(shadow()?.querySelectorAll("[data-line]").length).toBeLessThan(100);
  expect(shadow()?.querySelector("[contenteditable]")).toBeNull();
  await page.elementLocator(gutter(2)!).click();
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("2-2"));
  await page.elementLocator(gutter(4)!).click({ modifiers: ["Shift"] });
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("2-4"));
  await page.getByRole("button", { name: "跳转行" }).click();
  await page.getByRole("textbox", { name: "跳转到行" }).fill("900");
  await userEvent.keyboard("{Enter}");
  await expect
    .poll(() => shadow()?.textContent ?? "")
    .toContain("const line900");
  await page.getByRole("button", { name: "文件内查找" }).click();
  await page.getByRole("textbox", { name: "查找" }).fill("line850");
  await expect.element(page.getByText("0 / 1")).toBeVisible();
  await page.getByRole("button", { name: "下一个匹配", exact: true }).click();
  await expect.element(page.getByText("1 / 1")).toBeVisible();
  await expect
    .poll(() => shadow()?.textContent ?? "")
    .toContain("const line850");
});

test("goto accepts N:column and scrolls the code horizontally", async () => {
  const long = `const ${"x".repeat(400)} = 1;`;
  const text = Array.from({ length: 1000 }, (_, i) =>
    i === 499 ? long : `line ${i + 1}`,
  ).join("\n");
  render(Harness, { text });
  await expect.poll(() => shadow()?.textContent ?? "").toContain("line 1");
  expect(scrollbar()?.classList.contains("hidden")).toBe(true);
  await page.getByRole("button", { name: "跳转行" }).click();
  await page.getByRole("textbox", { name: "跳转到行" }).fill("500:200");
  await userEvent.keyboard("{Enter}");
  await expect.poll(() => row(500)?.textContent ?? "").toContain("xxxx");
  await expect.poll(() => code()?.scrollLeft ?? 0).toBeGreaterThan(0);
  await expect
    .poll(() => scrollbar()?.classList.contains("hidden") ?? true)
    .toBe(false);
  await expect
    .poll(() => scrollbar()?.scrollLeft ?? 0)
    .toBe(code()?.scrollLeft ?? -1);
  scrollbar()!.scrollLeft = 0;
  await expect.poll(() => code()?.scrollLeft ?? -1).toBe(0);
});

test("a goto submitted before the first placement survives it", async () => {
  const text = Array.from(
    { length: 400 },
    (_, i) => `const line${i + 1} = ${i + 1};`,
  ).join("\n");
  // A hidden reader has no layout, so its first placement waits until shown.
  const harness = await render(Harness, { text, hidden: true });
  document
    .querySelector<HTMLButtonElement>('button[aria-label="跳转行"]')!
    .click();
  await expect
    .poll(() => document.querySelector("[data-code-goto-line]"))
    .toBeTruthy();
  const input = document.querySelector<HTMLInputElement>(
    'input[aria-label="跳转到行"]',
  )!;
  input.value = "200";
  input.dispatchEvent(new Event("input", { bubbles: true }));
  document
    .querySelector<HTMLFormElement>("[data-code-goto-line]")!
    .requestSubmit();
  await harness.rerender({ hidden: false });
  const host = document.querySelector<HTMLElement>("[data-code-reader]")!;
  const inView = (line: number) => {
    const element = row(line);
    if (!element) return false;
    const rect = element.getBoundingClientRect();
    const box = host.getBoundingClientRect();
    return rect.top >= box.top && rect.bottom <= box.bottom;
  };
  await expect.poll(() => inView(200)).toBe(true);
  expect(harness.component.line()).toBeGreaterThan(150);
  expect(row(1)).toBeFalsy();
});

test("a relative goto before the first placement counts from the restored selection and keeps the restored state", async () => {
  const text = Array.from(
    { length: 400 },
    (_, i) => `const line${i + 1} = ${i + 1};`,
  ).join("\n");
  const search = {
    search: "line250",
    caseSensitive: false,
    regexp: false,
    wholeWord: false,
  };
  const harness = await render(Harness, {
    text,
    hidden: true,
    restore: {
      line: 10,
      offset: 0,
      selection: { start: 180, end: 182 },
      search,
    },
  });
  document
    .querySelector<HTMLButtonElement>('button[aria-label="跳转行"]')!
    .click();
  await expect
    .poll(() => document.querySelector("[data-code-goto-line]"))
    .toBeTruthy();
  const input = document.querySelector<HTMLInputElement>(
    'input[aria-label="跳转到行"]',
  )!;
  input.value = "+20";
  input.dispatchEvent(new Event("input", { bubbles: true }));
  document
    .querySelector<HTMLFormElement>("[data-code-goto-line]")!
    .requestSubmit();
  // Until layout arrives the restored state, now aimed at the jump, is kept.
  expect(harness.component.capture()).toMatchObject({
    line: 200,
    selection: { start: 180, end: 182 },
    search,
  });
  await harness.rerender({ hidden: false });
  await expect.poll(() => row(200)).toBeTruthy();
  const host = document.querySelector<HTMLElement>("[data-code-reader]")!;
  await expect
    .poll(() => {
      const rect = row(200)!.getBoundingClientRect();
      const box = host.getBoundingClientRect();
      return rect.top >= box.top && rect.bottom <= box.bottom;
    })
    .toBe(true);
  expect(harness.component.capture()).toMatchObject({
    selection: { start: 180, end: 182 },
    search,
  });
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("180-182"));
});

test("renders HTML and control-looking content as code text", async () => {
  const text =
    '<img src="invalid" onerror="window.codeExecuted=true">\n<script>window.codeExecuted=true</script>\ncarriage\rreturn\x1b[31m text';
  render(Harness, { text, path: "file.html" });
  await expect
    .poll(() => shadow()?.textContent ?? "")
    .toContain("window.codeExecuted");
  expect(shadow()?.querySelector("img[src], [onerror], script")).toBeNull();
  expect(shadow()?.querySelectorAll("[data-line]")).toHaveLength(4);
  expect(
    (window as unknown as { codeExecuted?: boolean }).codeExecuted,
  ).toBeUndefined();
});

test("keeps syntax highlighting across language and view extensions", async () => {
  render(Harness, {
    text: "const answer = 42; // highlighted",
    path: "test.ts",
  });
  await expect
    .poll(() => tokenColor("const"), { timeout: 10_000 })
    .not.toBe("");
  const keyword = tokenColor("const");
  const comment = tokenColor("// highlighted");
  expect(keyword).toMatch(/^rgb\(/);
  expect(comment).toMatch(/^rgb\(/);
  expect(keyword).not.toBe(comment);
});

test("highlights languages matched by extension and filename", async () => {
  render(Harness, {
    text: "name: webmux # highlighted",
    path: "compose.yaml",
  });
  await expect
    .poll(() => tokenColor("# highlighted"), { timeout: 10_000 })
    .not.toBe("");
  await cleanup();
  render(Harness, {
    text: "FROM alpine # highlighted",
    path: "Dockerfile",
  });
  await expect.poll(() => tokenColor("FROM"), { timeout: 10_000 }).not.toBe("");
});

test("retries uppercase extensions as lowercase", async () => {
  render(Harness, {
    text: "const answer = 42;",
    path: "FILE.TS",
  });
  await expect
    .poll(() => tokenColor("const"), { timeout: 10_000 })
    .not.toBe("");
});

test("keeps highlighting files larger than the old tokenize cutoff", async () => {
  const text = [
    "const answer = 42;",
    ...Array.from({ length: 4200 }, (_, i) => `line ${i + 1}`),
  ].join("\n");
  render(Harness, { text, path: "large.ts" });
  await expect
    .poll(() => tokenColor("const"), { timeout: 20_000 })
    .not.toBe("");
});

test("keeps Svelte script highlighting on JavaScript", async () => {
  render(Harness, {
    text: "const answer = 42;",
    path: "App.svelte",
  });
  await expect
    .poll(() => tokenColor("const"), { timeout: 10_000 })
    .not.toBe("");
});

test("captured reading state restores in a new reader, also one mounted hidden", async () => {
  const text = Array.from(
    { length: 1000 },
    (_, i) => `const line${i + 1} = ${i + 1};`,
  ).join("\n");
  const first = await render(Harness, { text });
  await expect
    .poll(() => shadow()?.textContent ?? "")
    .toContain("const line1 = 1;");
  await page.getByRole("button", { name: "文件内查找" }).click();
  await page.getByRole("textbox", { name: "查找" }).fill("line250");
  await page.getByRole("button", { name: "跳转行" }).click();
  await page.getByRole("textbox", { name: "跳转到行" }).fill("210");
  await userEvent.keyboard("{Enter}");
  await expect
    .poll(() => row(210)?.textContent ?? "")
    .toContain("const line210");
  await page.elementLocator(gutter(210)!).click();
  await page.elementLocator(gutter(215)!).click({ modifiers: ["Shift"] });
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("210-215"));
  // Selecting scrolls the clicked gutter into view, so jump back to line 200
  // before nudging it past the top edge for a captured offset.
  await page.getByRole("button", { name: "跳转行" }).click();
  await page.getByRole("textbox", { name: "跳转到行" }).fill("200");
  await userEvent.keyboard("{Enter}");
  const host = document.querySelector<HTMLElement>("[data-code-reader]")!;
  // goto queues its scroll on a frame. Nudging line 200 before that render
  // lands lets the queued center scroll overwrite the nudge, so wait until the
  // row reaches the requested centered position first.
  const centered = () => {
    const line = row(200);
    if (!line) return false;
    const box = host.getBoundingClientRect();
    const rect = line.getBoundingClientRect();
    return rect.top + rect.height / 2 >= box.top + box.height / 2 - rect.height;
  };
  await expect.poll(centered).toBe(true);
  const scrollIntoLine = () => {
    const line = row(200);
    if (!line) return false;
    host.scrollTop +=
      line.getBoundingClientRect().top - host.getBoundingClientRect().top + 11;
    return true;
  };
  await expect.poll(scrollIntoLine).toBe(true);
  await page.getByRole("button", { name: "文件内查找" }).click();
  await expect.poll(() => first.component.line()).toBeGreaterThan(150);
  const captured = first.component.capture();
  expect(Math.abs(captured.line - 200)).toBeLessThanOrEqual(2);
  expect(captured.offset).toBeGreaterThan(0);
  expect(captured).toMatchObject({
    selection: { start: 210, end: 215 },
    search: { search: "line250" },
    panel: "search",
  });
  await first.unmount();

  // A hidden tab has no layout: the state waits, and capture returns it.
  const second = await render(Harness, {
    text,
    restore: captured,
    hidden: true,
  });
  expect(second.component.capture()).toEqual(captured);
  await second.rerender({ hidden: false });
  await expect.poll(() => second.component.line()).toBe(captured.line);
  const restored = second.component.capture();
  expect(restored).toMatchObject({ ...captured, offset: expect.any(Number) });
  expect(restored.offset).toBeCloseTo(captured.offset, 1);
  await expect
    .element(page.getByRole("textbox", { name: "查找" }))
    .toHaveValue("line250");
  expect(document.activeElement?.closest("[data-code-search]")).toBeFalsy();
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("210-215"));
  await second.unmount();

  // Restoring into a visible reader applies at once.
  const third = await render(Harness, { text, restore: restored });
  await expect.poll(() => third.component.line()).toBe(restored.line);
  expect(third.component.capture()).toMatchObject({
    line: restored.line,
    selection: restored.selection,
    search: restored.search,
    panel: "search",
  });
});
