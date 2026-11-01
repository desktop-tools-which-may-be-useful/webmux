import { afterEach, expect, test } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { EditorView } from "@codemirror/view";
import Harness from "./CodeReaderHarness.svelte";

const editor = () =>
  EditorView.findFromDOM(document.querySelector<HTMLElement>(".cm-editor")!)!;
afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
});
test("read-only virtualized full file, real line selection, search and goto", async () => {
  const text = Array.from(
    { length: 1000 },
    (_, i) => `const line${i + 1} = ${i + 1};`,
  ).join("\n");
  render(Harness, { text });
  await expect
    .element(page.getByLabelText("只读代码"))
    .toMatchTextContent(new RegExp("const line1 = 1;"));
  expect(document.querySelectorAll(".cm-line").length).toBeLessThan(100);
  expect(
    document.querySelector(".cm-content")?.getAttribute("contenteditable"),
  ).toBe("false");
  const gutter = page.elementLocator(
    document.querySelector(".cm-lineNumbers")!,
  );
  await gutter.getByText("2", { exact: true }).click();
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("2-2"));
  await gutter.getByText("4", { exact: true }).click({ modifiers: ["Shift"] });
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("2-4"));
  await page.getByRole("button", { name: "跳转行" }).click();
  await page.getByRole("textbox", { name: "跳转到行" }).fill("900");
  await userEvent.keyboard("{Enter}");
  await expect
    .element(page.getByLabelText("只读代码"))
    .toMatchTextContent(new RegExp("const line900 = 900;"));
  await page.getByRole("button", { name: "文件内查找" }).click();
  await page.getByRole("textbox", { name: "查找" }).fill("line850");
  await page.getByRole("button", { name: "下一个匹配", exact: true }).click();
  await expect
    .element(page.getByLabelText("只读代码"))
    .toMatchTextContent(new RegExp("const line850 = 850;"));
});
test("renders HTML and control-looking content as code text", async () => {
  const text =
    '<img src="invalid" onerror="window.codeExecuted=true">\n<script>window.codeExecuted=true</script>\ncarriage\rreturn\x1b[31m text';
  render(Harness, { text, path: "file.html" });
  await expect
    .element(page.getByLabelText("只读代码"))
    .toMatchTextContent(new RegExp("window.codeExecuted"));
  expect(
    document.querySelector(
      '[data-code-reader] img[src="invalid"], [data-code-reader] [onerror], [data-code-reader] script',
    ),
  ).toBeNull();
  expect(
    (window as unknown as { codeExecuted?: boolean }).codeExecuted,
  ).toBeUndefined();
  expect(document.querySelectorAll(".cm-line")).toHaveLength(3);
});

test("keeps syntax highlighting across language and view extensions", async () => {
  render(Harness, {
    text: "const answer = 42; // highlighted",
    path: "test.ts",
  });
  const keyword = page
    .elementLocator(document.querySelector(".cm-content")!)
    .getByText("const", { exact: true });
  await expect.element(keyword).toBeVisible();
  await expect
    .poll(() => getComputedStyle(keyword.element()).color)
    .toBe("rgb(119, 0, 136)");
  const comment = page
    .elementLocator(document.querySelector(".cm-content")!)
    .getByText("// highlighted", { exact: true });
  await expect
    .poll(() => getComputedStyle(comment.element()).color)
    .toBe("rgb(153, 68, 0)");
});

test("highlights languages matched by extension and filename", async () => {
  render(Harness, {
    text: "name: webmux # highlighted",
    path: "compose.yaml",
  });
  const yamlComment = page
    .elementLocator(document.querySelector(".cm-content")!)
    .getByText("# highlighted", { exact: true });
  await expect
    .poll(() => getComputedStyle(yamlComment.element()).color)
    .toBe("rgb(153, 68, 0)");
  await cleanup();
  render(Harness, {
    text: "FROM alpine # highlighted",
    path: "Dockerfile",
  });
  const directive = page
    .elementLocator(document.querySelector(".cm-content")!)
    .getByText("FROM", { exact: true });
  await expect
    .poll(() => getComputedStyle(directive.element()).color)
    .toBe("rgb(119, 0, 136)");
});

test("keeps Svelte script highlighting on JavaScript", async () => {
  render(Harness, {
    text: "const answer = 42;",
    path: "App.svelte",
  });
  const keyword = page
    .elementLocator(document.querySelector(".cm-content")!)
    .getByText("const", { exact: true });
  await expect
    .poll(() => getComputedStyle(keyword.element()).color)
    .toBe("rgb(119, 0, 136)");
});

test("captured reading state restores in a new reader, also one mounted hidden", async () => {
  const text = Array.from(
    { length: 1000 },
    (_, i) => `const line${i + 1} = ${i + 1};`,
  ).join("\n");
  const first = await render(Harness, { text });
  await expect
    .element(page.getByLabelText("只读代码"))
    .toMatchTextContent(/const line1 = 1;/);
  await page.getByRole("button", { name: "文件内查找" }).click();
  await page.getByRole("textbox", { name: "查找" }).fill("line250");
  const view = editor();
  const { doc } = view.state;
  view.dispatch({
    selection: { anchor: doc.line(210).from, head: doc.line(215).to },
  });
  // Scroll halfway into line 200. Heights far from the viewport are
  // estimates, so settle once the line has been measured.
  const scrollIntoLine = () => {
    const block = view.lineBlockAt(doc.line(200).from);
    view.scrollDOM.scrollTop +=
      block.top +
      block.height / 2 -
      (view.scrollDOM.getBoundingClientRect().top - view.documentTop);
  };
  scrollIntoLine();
  await expect.poll(() => first.component.line()).toBe(200);
  scrollIntoLine();
  await expect.poll(() => first.component.capture().offset).toBeCloseTo(0.5, 1);
  const captured = first.component.capture();
  expect(captured).toMatchObject({
    line: 200,
    selection: {
      ranges: [{ anchor: doc.line(210).from, head: doc.line(215).to }],
      main: 0,
    },
    search: { search: "line250" },
    searchOpen: true,
    gotoOpen: false,
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
  await expect.poll(() => second.component.line()).toBe(200);
  const restored = second.component.capture();
  expect(restored).toMatchObject({ ...captured, offset: expect.any(Number) });
  expect(restored.offset).toBeCloseTo(captured.offset, 1);
  await expect
    .element(page.getByRole("textbox", { name: "查找" }))
    .toHaveValue("line250");
  expect(document.activeElement?.closest(".cm-panels")).toBeFalsy();
  const range = editor().state.selection.main;
  expect([range.from, range.to]).toEqual([
    doc.line(210).from,
    doc.line(215).to,
  ]);
  await second.unmount();

  // Restoring into a visible reader applies at once.
  const third = await render(Harness, { text, restore: restored });
  await expect.poll(() => third.component.line()).toBe(200);
  expect(third.component.capture()).toMatchObject({
    line: 200,
    selection: restored.selection,
    search: restored.search,
    searchOpen: true,
  });
});
