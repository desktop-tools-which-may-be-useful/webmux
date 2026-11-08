import { afterEach, expect, test, vi } from "vitest";
import { page } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./TerminalViewportHarness.svelte";

afterEach(cleanup);

test("touch gamepad renders only when enabled and controlling, supports slide-to-steer and repeat, and deduplicates bottom soft keys", async () => {
  const send = vi.fn();
  const toggle = vi.fn();
  const view = await render(Harness, {
    connection: "connected",
    restoring: false,
    controlling: false,
    touchGamepad: true,
    send,
    toggle,
  });

  // Not controlling: gamepad is hidden and bottom row keeps full keys.
  expect(document.querySelector('[aria-label="终端浮动摇杆"]')).toBeNull();
  expect(
    [...document.querySelectorAll(".soft-row button")].some(
      (b) => b.textContent === "Esc",
    ),
  ).toBe(true);

  // Claiming control reveals the semi-transparent gamepad and trims bottom soft keys.
  await view.rerender({
    connection: "connected",
    restoring: false,
    controlling: true,
    touchGamepad: true,
    send,
    toggle,
  });
  const gamepad = document.querySelector<HTMLElement>(
    '[aria-label="终端浮动摇杆"]',
  );
  expect(gamepad).not.toBeNull();
  expect(gamepad!.classList.contains("pointer-events-none")).toBe(true);
  expect(
    [...document.querySelectorAll(".soft-row button")].some(
      (b) => b.textContent === "Esc",
    ),
  ).toBe(false);

  const key = (name: string) =>
    gamepad!.querySelector<HTMLButtonElement>(`[data-gamepad-key="${name}"]`)!;

  key("esc").click();
  expect(send).toHaveBeenCalledWith("\x1b");
  key("tab").click();
  expect(send).toHaveBeenCalledWith("\t");
  key("ctrl-c").click();
  expect(send).toHaveBeenCalledWith("\x03");
  key("enter").click();
  expect(send).toHaveBeenCalledWith("\r");
  key("space").click();
  expect(send).toHaveBeenCalledWith(" ");

  // Touching/scrolling the terminal host fades the gamepad into semi-transparent state.
  const host = document.querySelector<HTMLElement>(".terminal-host")!;
  expect(gamepad!.getAttribute("data-faded")).toBeNull();
  host.dispatchEvent(
    new PointerEvent("pointerdown", {
      bubbles: true,
      cancelable: true,
      pointerId: 10,
    }),
  );
  await expect.poll(() => gamepad!.getAttribute("data-faded")).toBe("true");
  // Touching a gamepad button immediately restores full HUD visibility.
  key("esc").dispatchEvent(
    new PointerEvent("pointerdown", {
      bubbles: true,
      cancelable: true,
      pointerId: 11,
    }),
  );
  await expect.poll(() => gamepad!.getAttribute("data-faded")).toBeNull();

  // Slide-to-steer on the D-Pad: press Up, then drag right past the threshold.
  send.mockClear();
  const up = key("up");
  up.dispatchEvent(
    new PointerEvent("pointerdown", {
      bubbles: true,
      cancelable: true,
      pointerId: 1,
      button: 0,
      clientX: 100,
      clientY: 100,
    }),
  );
  expect(send).toHaveBeenLastCalledWith("\x1b[A");
  up.dispatchEvent(
    new PointerEvent("pointermove", {
      bubbles: true,
      cancelable: true,
      pointerId: 1,
      clientX: 130,
      clientY: 100,
    }),
  );
  expect(send).toHaveBeenLastCalledWith("\x1b[C");
  up.dispatchEvent(
    new PointerEvent("pointerup", {
      bubbles: true,
      cancelable: true,
      pointerId: 1,
    }),
  );

  // Hold auto-repeat on the D-Pad.
  send.mockClear();
  up.dispatchEvent(
    new PointerEvent("pointerdown", {
      bubbles: true,
      cancelable: true,
      pointerId: 2,
      button: 0,
      clientX: 100,
      clientY: 100,
    }),
  );
  expect(send).toHaveBeenCalledTimes(1);
  expect(send).toHaveBeenLastCalledWith("\x1b[A");
  await new Promise((resolve) => setTimeout(resolve, 480));
  expect(send.mock.calls.length).toBeGreaterThanOrEqual(2);
  up.dispatchEvent(
    new PointerEvent("pointerup", {
      bubbles: true,
      cancelable: true,
      pointerId: 2,
    }),
  );
});

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
