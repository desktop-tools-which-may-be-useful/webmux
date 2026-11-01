import type { Terminal } from "@xterm/xterm";

interface RendererInternals {
  renderRows?: (start: number, end: number) => void;
  clear?: () => void;
  // Only xterm's DOM renderer keeps one element per row.
  _rowElements?: unknown;
}

interface CoreInternals {
  _renderService?: {
    setRenderer?: (renderer: RendererInternals | undefined) => void;
    _renderer?: { value?: RendererInternals };
  };
  coreService?: { decPrivateModes?: { synchronizedOutput?: unknown } };
}

function core(term: Terminal) {
  return (term as unknown as { _core?: CoreInternals })._core;
}

// xterm 6.0 gates its repaints on DEC 2026 synchronized output, but its DOM
// renderer also repaints rows on its own: for the selection refresh that every
// scroll queues, for a resize, focus or cursor change, and it empties every row
// when a buffer is reset or switched. A synchronized redraw that scrolls, such
// as a TUI reprinting its transcript, or a replay after reset() then showed its
// partial or blank frames anyway. While an update is synchronized, the DOM
// renderer's own row repaints and clears become a buffered refresh, which
// xterm paints once when the update ends. The WebGL renderer already routes
// every repaint through the gated refresh and keeps its own glyph model, so it
// is left untouched. Internals are feature-detected; without them xterm keeps
// its own behavior.
export function holdRepaintsDuringSync(term: Terminal): () => void {
  const service = core(term)?._renderService;
  const setRenderer = service?.setRenderer;
  if (!service || typeof setRenderer !== "function") return () => {};
  const synchronized = () => term.modes.synchronizedOutputMode;
  const restores: (() => void)[] = [];
  const refresh = (start: number, end: number) =>
    term.refresh(Math.max(0, start), Math.min(term.rows - 1, end));

  const hold = (renderer: RendererInternals | undefined) => {
    if (!renderer || !Array.isArray(renderer._rowElements)) return;
    const { renderRows, clear } = renderer;
    if (typeof renderRows !== "function" || typeof clear !== "function") return;
    const heldRows = (start: number, end: number) => {
      if (synchronized()) refresh(start, end);
      else renderRows.call(renderer, start, end);
    };
    const heldClear = () => {
      if (synchronized()) refresh(0, term.rows - 1);
      else clear.call(renderer);
    };
    renderer.renderRows = heldRows;
    renderer.clear = heldClear;
    restores.push(() => {
      if (renderer.renderRows === heldRows) renderer.renderRows = renderRows;
      if (renderer.clear === heldClear) renderer.clear = clear;
    });
  };
  hold(service._renderer?.value);
  // The DOM renderer is replaced when WebGL loads or falls back.
  const heldSetRenderer: typeof setRenderer = (renderer) => {
    hold(renderer);
    setRenderer.call(service, renderer);
  };
  service.setRenderer = heldSetRenderer;

  return () => {
    if (service.setRenderer === heldSetRenderer)
      service.setRenderer = setRenderer;
    for (const restore of restores.splice(0)) restore();
  };
}

// Ends a synchronized update without writing bytes into the output stream:
// a replayed snapshot may stop inside an unfinished APC, OSC or DCS sequence,
// which would swallow a written `CSI ? 2026 l` as data. The modes object is
// replaced by reset(), so it is looked up when the update ends. Returns false
// when the internals are unavailable.
export function endSynchronizedOutput(term: Terminal): boolean {
  const modes = core(term)?.coreService?.decPrivateModes;
  if (!modes || typeof modes.synchronizedOutput !== "boolean") return false;
  if (modes.synchronizedOutput) {
    modes.synchronizedOutput = false;
    // Paints the rows buffered while synchronized and clears xterm's timeout.
    term.refresh(0, term.rows - 1);
  }
  return true;
}

// Whether endSynchronizedOutput() can end an update this terminal starts.
export function canEndSynchronizedOutput(term: Terminal): boolean {
  const modes = core(term)?.coreService?.decPrivateModes;
  return !!modes && typeof modes.synchronizedOutput === "boolean";
}
