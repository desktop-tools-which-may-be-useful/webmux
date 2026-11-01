import type { Attachment } from "svelte/attachments";

// Mirrors the soft-key long press: hold time and the movement that turns a
// press into a scroll.
const holdMs = 500;
const moveTolerance = 8;
// How long after the finger lifts a compatibility click may still arrive.
const ghostClickMs = 800;

/**
 * Calls press after a touch or pen is held on the element, or with a
 * selector on the matching descendant where the press started (one listener
 * for a whole list, unaffected by its rows re-rendering). iOS Safari fires
 * no contextmenu event for a long press, so touch context actions need their
 * own gesture.
 *
 * The click that ends a long press is swallowed anywhere in the document:
 * whatever press opened (a sheet sliding up under the finger) must not
 * receive it. The browser's own long-press menu (Android) is suppressed
 * while a press is held.
 */
export function longPress(
  press: (element: HTMLElement) => void,
  selector?: string,
): Attachment<HTMLElement> {
  return (element) => {
    const document = element.ownerDocument;
    let timer: number | undefined;
    let origin: { x: number; y: number } | undefined;
    let fired = false;
    let ghostTimer: number | undefined;
    const stop = () => {
      clearTimeout(timer);
      timer = undefined;
      origin = undefined;
    };
    const swallow = (event: MouseEvent) => {
      event.preventDefault();
      event.stopImmediatePropagation();
      release();
    };
    const release = () => {
      clearTimeout(ghostTimer);
      document.removeEventListener("click", swallow, true);
    };
    const down = (event: PointerEvent) => {
      if (event.pointerType === "mouse" || !event.isPrimary) return;
      const target = selector
        ? (event.target as Element | null)?.closest<HTMLElement>(selector)
        : element;
      if (!target || !element.contains(target)) return;
      stop();
      release();
      fired = false;
      origin = { x: event.clientX, y: event.clientY };
      timer = window.setTimeout(() => {
        timer = undefined;
        fired = true;
        navigator.vibrate?.(10);
        press(target);
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
    const up = () => {
      stop();
      if (!fired) return;
      fired = false;
      document.addEventListener("click", swallow, true);
      ghostTimer = window.setTimeout(release, ghostClickMs);
    };
    const menu = (event: Event) => {
      if (origin || fired) event.preventDefault();
    };
    element.addEventListener("pointerdown", down);
    element.addEventListener("pointermove", move);
    element.addEventListener("pointercancel", stop);
    element.addEventListener("contextmenu", menu);
    // The finger may lift over the sheet that press opened, outside element.
    document.addEventListener("pointerup", up, true);
    return () => {
      stop();
      release();
      element.removeEventListener("pointerdown", down);
      element.removeEventListener("pointermove", move);
      element.removeEventListener("pointercancel", stop);
      element.removeEventListener("contextmenu", menu);
      document.removeEventListener("pointerup", up, true);
    };
  };
}
