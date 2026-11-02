import { tick } from "svelte";
import type {
  CodeView,
  CodeViewOptions,
  SelectedLineRange,
} from "@pierre/diffs";
import { observeDarkTheme } from "./code-pierre";
import {
  stepMatchLine,
  type MatchCursor,
  type SearchSpec,
} from "./code-search";

type Panel = "search" | "goto";

/** Focuses the open search or go-to-line input after it renders. */
async function focusCodePanel(root: ParentNode) {
  await tick();
  root
    .querySelector<HTMLInputElement>(
      "[data-code-search] input, [data-code-goto-line] input",
    )
    ?.focus();
}

/** Toggles the named panel, or closes it when it is already open. */
function togglePanel<Panel extends string>(
  current: Panel | undefined,
  value: Panel,
  toggle: boolean,
): Panel | undefined {
  return toggle && current === value ? undefined : value;
}

/** Opens or closes a reader panel and focuses its input. */
async function openCodePanel(
  root: ParentNode,
  current: "search" | "goto" | undefined,
  value: "search" | "goto",
  toggle: boolean,
  setPanel: (panel: "search" | "goto" | undefined) => void,
) {
  setPanel(togglePanel(current, value, toggle));
  await focusCodePanel(root);
}

/** Closes a reader panel and returns focus to its scroll surface. */
function closeCodePanel(
  host: HTMLElement,
  setPanel: (panel: undefined) => void,
) {
  setPanel(undefined);
  host.focus();
}

/** Panel toggling, search edits and match stepping for a code reader. */
export function createReaderPanelActions(options: {
  root: () => ParentNode;
  host: () => HTMLElement;
  getPanel: () => Panel | undefined;
  setPanel: (panel: Panel | undefined) => void;
  getCursor: () => MatchCursor | undefined;
  setCursor: (cursor: MatchCursor | undefined) => void;
  setSearch: (search: SearchSpec) => void;
  getMatches: () => readonly number[];
  reveal: (line: number) => void;
  // Diff readers have no searchable lines until their diff is ready.
  enabled?: () => boolean;
}) {
  const actions = {
    async openPanel(value: Panel, toggle = true) {
      if (options.enabled && !options.enabled()) return;
      await openCodePanel(
        options.root(),
        options.getPanel(),
        value,
        toggle,
        options.setPanel,
      );
    },
    query(value: SearchSpec) {
      options.setSearch(value);
      options.setCursor(undefined);
    },
    closePanel() {
      closeCodePanel(options.host(), options.setPanel);
    },
    nextMatch(direction: 1 | -1) {
      options.setCursor(
        advanceMatch(
          options.getCursor(),
          options.getMatches(),
          direction,
          options.reveal,
        ),
      );
    },
  };
  return {
    ...actions,
    keyHandlers: readerKeyHandlers(
      (value, toggle) => actions.openPanel(value, toggle),
      (direction) => actions.nextMatch(direction),
      () => actions.closePanel(),
    ),
  };
}

/** Key handlers shared by both readers; `openPanel` opens without toggling. */
function readerKeyHandlers(
  openPanel: (value: "search" | "goto", toggle?: boolean) => unknown,
  nextMatch: (direction: 1 | -1) => void,
  close: () => void,
) {
  return {
    openSearch: () => void openPanel("search", false),
    openGoto: () => void openPanel("goto", false),
    nextMatch,
    close,
  };
}

/** Shared search and navigation keys for both code readers. */
export function readerKeydown(
  event: KeyboardEvent,
  handlers: {
    openSearch: () => void;
    openGoto: () => void;
    nextMatch: (direction: 1 | -1) => void;
    close: () => void;
  },
) {
  const mod = event.metaKey || event.ctrlKey;
  const key = event.key.toLowerCase();
  if (mod && key === "f") {
    event.preventDefault();
    handlers.openSearch();
  } else if (mod && event.altKey && key === "g") {
    event.preventDefault();
    handlers.openGoto();
  } else if (event.key === "F3" || (mod && key === "g")) {
    event.preventDefault();
    handlers.nextMatch(event.shiftKey ? -1 : 1);
  } else if (event.key === "Escape") {
    handlers.close();
  }
}

/**
 * Measures on every scroll event, not on a later frame: a tab can be hidden
 * before the next frame, and its last position must already be captured.
 */
function watchScroll(host: HTMLElement, measure: () => void) {
  host.addEventListener("scroll", measure, { passive: true });
  return () => host.removeEventListener("scroll", measure);
}

/** Pointer, theme, resize and scroll wiring shared by the code readers. */
export function watchReader(
  host: HTMLElement,
  handlers: {
    onPointer: (coarse: boolean) => void;
    onTheme: (dark: boolean) => void;
    onResize: () => void;
    onScroll: () => void;
  },
) {
  const pointer = matchMedia("(pointer: coarse)");
  const updatePointer = () => handlers.onPointer(pointer.matches);
  updatePointer();
  pointer.addEventListener("change", updatePointer);
  const stopTheme = observeDarkTheme(handlers.onTheme);
  const resize = new ResizeObserver(handlers.onResize);
  resize.observe(host);
  const stopScroll = watchScroll(host, handlers.onScroll);
  return () => {
    resize.disconnect();
    stopTheme();
    pointer.removeEventListener("change", updatePointer);
    stopScroll();
  };
}

/** Moves to the next match, reveals its line and returns the new cursor. */
function advanceMatch(
  cursor: MatchCursor | undefined,
  matches: readonly number[],
  direction: 1 | -1,
  reveal: (line: number) => void,
) {
  const next = stepMatchLine(cursor, matches, direction);
  if (!next) return cursor;
  reveal(next.line);
  return next.cursor;
}

/** Releases a reader's rendered DOM and its shared highlighter. */
export function releaseCodeView(
  viewer: CodeView | undefined,
  highlighter: { release: () => void },
) {
  viewer?.cleanUp();
  highlighter.release();
}

/** Re-renders a reader with new options and keeps its line selection. */
export function syncViewerOptions(
  viewer: CodeView | undefined,
  options: CodeViewOptions<undefined>,
  onSelection?: (range: SelectedLineRange) => void,
) {
  if (!viewer) return;
  const selection = viewer.getSelectedLines();
  viewer.setOptions(options);
  viewer.render();
  if (selection) {
    viewer.setSelectedLines(selection, { notify: false });
    onSelection?.(selection.range);
  }
}
