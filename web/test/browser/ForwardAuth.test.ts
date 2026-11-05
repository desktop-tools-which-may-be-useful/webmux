import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { page } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import ForwardAuth from "../../src/pages/ForwardAuth.svelte";
import { stubFetch } from "./forward-fixture";

let address = "";
beforeEach(() => {
  address = location.pathname + location.search;
});
afterEach(async () => {
  try {
    await cleanup();
  } finally {
    history.replaceState(history.state, "", address);
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    for (const added of document.querySelectorAll(
      'form, meta[name="referrer"]',
    ))
      added.remove();
  }
});

function visit(next: string) {
  history.replaceState(
    history.state,
    "",
    `${location.pathname}?next=${encodeURIComponent(next)}`,
  );
}

test("a signed-in tab trades a ticket through a top-level form POST", async () => {
  visit("/forward/5173/app?page=2");
  const fetch = stubFetch(() =>
    Response.json(
      { code: "one-time-code", expiresAt: "2026-11-05T10:01:00Z" },
      { status: 201 },
    ),
  );
  const submit = vi
    .spyOn(HTMLFormElement.prototype, "submit")
    .mockImplementation(() => {});
  await render(ForwardAuth);
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("正在打开转发…");
  await vi.waitFor(() => expect(submit).toHaveBeenCalledOnce());

  const [url, init] = fetch.mock.calls[0]!;
  expect(String(url)).toBe("/api/forwards/tickets");
  expect(init?.method).toBe("POST");
  const form = submit.mock.contexts[0] as HTMLFormElement;
  expect(form.method).toBe("post");
  expect(new URL(form.action).pathname).toBe("/forward/_auth");
  expect(Object.fromEntries(new FormData(form))).toEqual({
    code: "one-time-code",
    next: "/forward/5173/app?page=2",
  });
  // The code never enters this page's address.
  expect(location.href).not.toContain("one-time-code");
  // Under the app's no-referrer policy the POST would carry `Origin: null`.
  expect(
    document.querySelector('meta[name="referrer"]')?.getAttribute("content"),
  ).toBe("same-origin");
});

test("a failed ticket shows the error and a way back", async () => {
  visit("/forward/5173/");
  stubFetch(() => Response.json({ error: "服务暂不可用" }, { status: 503 }));
  const submit = vi.spyOn(HTMLFormElement.prototype, "submit");
  await render(ForwardAuth);
  await expect
    .element(page.getByRole("alert"))
    .toHaveTextContent("无法打开转发：服务暂不可用");
  await expect
    .element(page.getByRole("link", { name: "返回 WebMux" }))
    .toHaveAttribute("href", "/");
  expect(submit).not.toHaveBeenCalled();
});

test("a return address outside the gateway is refused without a ticket", async () => {
  visit("//evil.example/");
  const fetch = stubFetch(() => Response.json({}));
  await render(ForwardAuth);
  await expect
    .element(page.getByRole("alert"))
    .toHaveTextContent("转发地址无效。");
  expect(fetch).not.toHaveBeenCalled();
});

test("a load another site started waits for a click before redeeming", async () => {
  history.replaceState(
    history.state,
    "",
    `${location.pathname}?confirm=1&next=${encodeURIComponent("/forward/5173/")}`,
  );
  const fetch = stubFetch(() =>
    Response.json(
      { code: "one-time-code", expiresAt: "2026-11-05T10:01:00Z" },
      { status: 201 },
    ),
  );
  const submit = vi
    .spyOn(HTMLFormElement.prototype, "submit")
    .mockImplementation(() => {});
  await render(ForwardAuth);
  await expect.element(page.getByText("/forward/5173/")).toBeVisible();
  await expect
    .element(page.getByRole("button", { name: "打开" }))
    .toBeVisible();
  expect(fetch).not.toHaveBeenCalled();

  await page.getByRole("button", { name: "打开" }).click();
  await vi.waitFor(() => expect(submit).toHaveBeenCalledOnce());
  expect(fetch).toHaveBeenCalledOnce();
});
