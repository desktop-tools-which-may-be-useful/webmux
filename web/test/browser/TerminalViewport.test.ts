import { afterEach, expect, test } from "vitest";
import { page } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./TerminalViewportHarness.svelte";

afterEach(cleanup);

test.each(["connecting", "reconnecting"] as const)(
  "a synced %s terminal shows a non-blocking restoration hint",
  async (connection) => {
    const view = await render(Harness, { connection, restoring: false });
    const hint = page.getByText("正在恢复…");
    await expect.element(hint).toBeVisible();
    // The hint floats over retained output without covering or intercepting it.
    expect(hint.element().closest(".pointer-events-none")).not.toBeNull();
    expect(document.querySelector(".restore-overlay")).toBeNull();

    await view.rerender({ connection: "connected", restoring: false });
    await expect.element(hint).not.toBeInTheDocument();
  },
);

test("restoration keeps its blocking overlay labels", async () => {
  const view = await render(Harness, {
    connection: "connecting",
    restoring: true,
  });
  await expect.element(page.getByText("正在连接…")).toBeVisible();
  expect(document.querySelector(".restore-overlay")).not.toBeNull();

  await view.rerender({ connection: "connected", restoring: true });
  await expect.element(page.getByText("正在恢复…")).toBeVisible();

  await view.rerender({ connection: "connected", restoring: false });
  expect(document.querySelector(".restore-overlay")).toBeNull();
});
