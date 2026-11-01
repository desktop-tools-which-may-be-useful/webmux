import type { Attachment } from "svelte/attachments";

// Mirrors the soft-key long press: hold time and the movement that turns a
// press into a scroll.
const holdMs = 500;
const moveTolerance = 8;

/**
 * Calls press after a touch or pen is held on the element. iOS Safari fires
 * no contextmenu event for a long press, so touch context actions need their
 * own gesture. The click that ends a long press is swallowed so the element
 * does not also activate, and the browser's own long-press menu (Android)
 * is suppressed while a press is held.
 */
export function longPress(
  press: (element: HTMLElement) => void,
): Attachment<HTMLElement> {
  return (element) => {
    let timer: number | undefined;
    let origin: { x: number; y: number } | undefined;
    let fired = false;
    const stop = () => {
      clearTimeout(timer);
      timer = undefined;
      origin = undefined;
    };
    const down = (event: PointerEvent) => {
      if (event.pointerType === "mouse" || !event.isPrimary) return;
      stop();
      fired = false;
      origin = { x: event.clientX, y: event.clientY };
      timer = window.setTimeout(() => {
        timer = undefined;
        fired = true;
        navigator.vibrate?.(10);
        press(element);
      }, holdMs);
    };
    const move = (event: PointerEvent) => {
      if (
        origin &&
        Math.hypot(event.clientX - origin.x, event.clientY - origin.y) >=
          moveTolerance
      )
        stop();
    };
    const click = (event: MouseEvent) => {
      if (!fired) return;
      fired = false;
      event.preventDefault();
      event.stopImmediatePropagation();
    };
    const menu = (event: Event) => {
      if (origin || fired) event.preventDefault();
    };
    element.addEventListener("pointerdown", down);
    element.addEventListener("pointermove", move);
    element.addEventListener("pointerup", stop);
    element.addEventListener("pointercancel", stop);
    element.addEventListener("click", click, { capture: true });
    element.addEventListener("contextmenu", menu);
    return () => {
      stop();
      element.removeEventListener("pointerdown", down);
      element.removeEventListener("pointermove", move);
      element.removeEventListener("pointerup", stop);
      element.removeEventListener("pointercancel", stop);
      element.removeEventListener("click", click, { capture: true });
      element.removeEventListener("contextmenu", menu);
    };
  };
}
