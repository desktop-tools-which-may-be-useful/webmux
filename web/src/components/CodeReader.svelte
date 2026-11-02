<script lang="ts">
  import { onMount, untrack } from "svelte";
  import {
    Compartment,
    EditorSelection,
    EditorState,
    StateEffect,
    StateField,
    Text,
    type Extension,
  } from "@codemirror/state";
  import {
    EditorView,
    lineNumbers,
    highlightActiveLine,
    highlightSpecialChars,
    Decoration,
  } from "@codemirror/view";
  import {
    getSearchQuery,
    openSearchPanel,
    searchPanelOpen,
  } from "@codemirror/search";
  import {
    codeReaderPanels,
    gotoLineOpen,
    openGotoLine,
    restorePanels,
  } from "./code-reader-panels";
  import { syntaxHighlighting } from "@codemirror/language";
  import { codeHighlightStyle } from "../code-highlight";
  import { loadCodeLanguage } from "../code-language";
  import { Button } from "$lib/components/ui/button";
  import IconListNumbers from "~icons/tabler/list-numbers";
  import IconNumber from "~icons/tabler/number";
  import IconSearch from "~icons/tabler/search";
  import { settings, changeSettings } from "../settings.svelte";
  import { defaultCodeReader } from "../settings-intent";
  import type { CodeReaderState } from "../schemas";
  let {
    text,
    path,
    onselect,
    restoreLine = 1,
    restore,
    draftEnabled = false,
    hint = $bindable(""),
  }: {
    text: string;
    path: string;
    onselect: (start: number, end: number) => void;
    restoreLine?: number;
    // A captured reading state, applied once the editor has layout; it takes
    // precedence over `restoreLine`.
    restore?: CodeReaderState;
    draftEnabled?: boolean;
    // Selection guidance, rendered by the document's action bar.
    hint?: string;
  } = $props();
  // Until applied, `capture()` returns it unchanged.
  let pending = untrack(() => restore);
  // The scroll position last measured while the editor had layout; a hidden
  // tab reports this one.
  let known: Pick<CodeReaderState, "line" | "offset"> | undefined;
  // Restoring sets the editor selection without reporting it as a new pick.
  let restoring = false;
  let host: HTMLDivElement;
  let view: EditorView | undefined;
  let anchor: number | undefined;
  let coarse = $state(false);
  let actionLine = $state<number>();
  const pickLine = StateEffect.define<number | undefined>();
  const gutterSlot = new Compartment();
  let lineGutter: Extension = [];
  const showGutter = $derived(
    (settings.values.codeReader ?? defaultCodeReader).lineNumbers === "show",
  );
  function toggleGutter() {
    changeSettings({
      codeReader: {
        ...(settings.values.codeReader ?? defaultCodeReader),
        lineNumbers: showGutter ? "hide" : "show",
      },
    });
  }
  $effect(() => {
    view?.dispatch({
      effects: gutterSlot.reconfigure(showGutter ? lineGutter : []),
    });
  });
  export function clearLineAction() {
    actionLine = undefined;
    view?.dispatch({ effects: pickLine.of(undefined) });
  }

  function position(): Pick<CodeReaderState, "line" | "offset"> {
    if (!view || host.clientHeight === 0)
      return known ?? { line: untrack(() => restoreLine), offset: 0 };
    const top = view.scrollDOM.getBoundingClientRect().top - view.documentTop;
    const block = view.lineBlockAtHeight(top);
    known = {
      line: view.state.doc.lineAt(block.from).number,
      offset:
        block.height > 0
          ? Math.min(1, Math.max(0, (top - block.top) / block.height))
          : 0,
    };
    return known;
  }
  export function line() {
    return position().line;
  }
  export function capture(): CodeReaderState {
    if (pending || !view) return pending ?? { ...empty(), ...position() };
    const { selection } = view.state;
    const query = getSearchQuery(view.state);
    return {
      ...position(),
      selection: {
        ranges: selection.ranges.map(({ anchor, head }) => ({ anchor, head })),
        main: selection.mainIndex,
      },
      search: {
        search: query.search,
        caseSensitive: query.caseSensitive,
        regexp: query.regexp,
        wholeWord: query.wholeWord,
      },
      searchOpen: searchPanelOpen(view.state),
      gotoOpen: gotoLineOpen(view.state),
      ...(actionLine === undefined ? {} : { picked: actionLine }),
    };
  }
  function empty(): CodeReaderState {
    return {
      line: 1,
      offset: 0,
      selection: { ranges: [{ anchor: 0, head: 0 }], main: 0 },
      search: {
        search: "",
        caseSensitive: false,
        regexp: false,
        wholeWord: false,
      },
      searchOpen: false,
      gotoOpen: false,
    };
  }
  // Applies `pending` once the editor has layout to scroll in.
  function applyRestore() {
    const state = pending;
    if (!view || !state || host.clientHeight === 0) return;
    pending = undefined;
    const { doc } = view.state;
    const at = (value: number) => Math.min(value, doc.length);
    const ranges = state.selection.ranges.map(({ anchor, head }) =>
      EditorSelection.range(at(anchor), at(head)),
    );
    const target = doc.line(Math.min(state.line, doc.lines));
    restoring = true;
    try {
      view.dispatch({
        selection: EditorSelection.create(
          ranges,
          Math.min(state.selection.main, ranges.length - 1),
        ),
        effects: EditorView.scrollIntoView(target.from, {
          y: "start",
          yMargin: -state.offset * view.lineBlockAt(target.from).height,
        }),
      });
      restorePanels(view, state);
      if (state.picked !== undefined && state.picked <= doc.lines && coarse) {
        actionLine = state.picked;
        view.dispatch({ effects: pickLine.of(state.picked) });
      }
    } finally {
      restoring = false;
    }
  }
  function select(number: number, shift: boolean) {
    if (coarse) {
      anchor = undefined;
      actionLine = number;
      if (view) {
        view.dispatch({
          selection: { anchor: view.state.doc.line(number).from },
          effects: pickLine.of(number),
        });
      }
      onselect(number, number);
      return;
    }
    if (shift && anchor !== undefined) {
      onselect(Math.min(anchor, number), Math.max(anchor, number));
      anchor = undefined;
    } else {
      anchor = number;
      onselect(number, number);
    }
  }
  onMount(() => {
    let cancelled = false;
    const pointer = matchMedia("(pointer: coarse)");
    const updatePointer = () => {
      coarse = pointer.matches;
      if (!coarse) clearLineAction();
    };
    updatePointer();
    pointer.addEventListener("change", updatePointer);
    const selectionChanged = () => {
      const selection = document.getSelection();
      if (
        selection &&
        !selection.isCollapsed &&
        host.contains(selection.anchorNode)
      ) {
        // A transaction here would reset the browser's native selection.
        actionLine = undefined;
      }
    };
    document.addEventListener("selectionchange", selectionChanged);
    lineGutter = lineNumbers({
      domEventHandlers: {
        click: (v, l, e) => {
          select(v.state.doc.lineAt(l.from).number, (e as MouseEvent).shiftKey);
          return true;
        },
      },
    });

    const extensions = [
      EditorState.readOnly.of(true),
      EditorView.editable.of(false),
      codeReaderPanels(),
      syntaxHighlighting(codeHighlightStyle),
      highlightActiveLine(),
      highlightSpecialChars(),
      EditorView.theme({
        "&": { height: "100%", fontSize: "13px" },
        ".cm-scroller": {
          overflow: "auto",
          fontFamily: "var(--font-mono, monospace)",
        },
        ".cm-content": { minWidth: "100%" },
        ".cm-gutters": {
          cursor: "pointer",
          backgroundColor: "var(--muted)",
          color: "var(--muted-foreground)",
          borderRight: "1px solid var(--border)",
        },
        ".cm-activeLineGutter": {
          backgroundColor: "var(--accent)",
          color: "var(--foreground)",
        },
        ".cm-panels": {
          backgroundColor: "var(--background)",
          color: "var(--foreground)",
        },
        ".cm-panels.cm-panels-bottom": { borderTop: "1px solid var(--border)" },
        ".cm-panels.cm-panels-top": { borderBottom: "1px solid var(--border)" },
        ".code-line-picked": {
          boxShadow: "inset 3px 0 var(--primary)",
          outline: "1px solid var(--primary)",
          outlineOffset: "-1px",
        },
      }),
      EditorView.domEventHandlers({
        click(event, editor) {
          if (!coarse) return false;
          const target = event.target;
          if (!(target instanceof Element) || !target.closest(".cm-line"))
            return false;
          if (document.getSelection()?.isCollapsed === false) return false;
          const pos = editor.posAtCoords({
            x: event.clientX,
            y: event.clientY,
          });
          if (pos === null) return false;
          const row = editor.state.doc.lineAt(pos).number;
          select(row, false);
          return false;
        },
        scroll() {
          if (!pending) position();
          return false;
        },
      }),
      gutterSlot.of(showGutter ? lineGutter : []),
      StateField.define({
        create: () => Decoration.none,
        update(value, transaction) {
          for (const effect of transaction.effects) {
            if (effect.is(pickLine)) {
              return effect.value === undefined
                ? Decoration.none
                : Decoration.set([
                    Decoration.line({ class: "code-line-picked" }).range(
                      transaction.state.doc.line(effect.value).from,
                    ),
                  ]);
            }
          }
          return value;
        },
        provide: (field) => EditorView.decorations.from(field),
      }),
      EditorView.updateListener.of((u) => {
        if (u.selectionSet) {
          const range = u.state.selection.main;
          if (!range.empty && !restoring)
            onselect(
              u.state.doc.lineAt(range.from).number,
              u.state.doc.lineAt(Math.max(range.from, range.to - 1)).number,
            );
        }
      }),
    ];
    view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: Text.of(text.split(/\r?\n/)),
        extensions,
      }),
    });
    // A hidden tab has no layout to scroll in; restore once it is shown.
    const shown = new ResizeObserver(() => {
      applyRestore();
      if (!pending) shown.disconnect();
    });
    if (pending) {
      applyRestore();
      if (pending) shown.observe(host);
    } else {
      const target = Math.min(Math.max(restoreLine, 1), view.state.doc.lines);
      view.dispatch({
        effects: EditorView.scrollIntoView(view.state.doc.line(target).from, {
          y: "start",
          yMargin: 0,
        }),
      });
    }
    void loadCodeLanguage(path)
      .then((language) => {
        if (!cancelled && view && language)
          view.dispatch({ effects: StateEffect.appendConfig.of(language) });
      })
      .catch(() => {
        /* Plain text remains readable if language loading fails. */
      });
    return () => {
      cancelled = true;
      shown.disconnect();
      pointer.removeEventListener("change", updatePointer);
      document.removeEventListener("selectionchange", selectionChanged);
      view?.destroy();
      view = undefined;
    };
  });
  $effect(() => {
    hint = coarse
      ? `点击代码行，再点右侧 ＋ 发送位置到${draftEnabled ? "草稿" : "终端"}`
      : showGutter
        ? "点击行号；Shift 点击选择终点"
        : "显示行号后可点击行号选择";
    return () => {
      hint = "";
    };
  });
</script>

<div class="flex min-h-0 flex-1 flex-col overflow-hidden">
  <div class="flex shrink-0 items-center gap-1 border-b px-1 py-0.5">
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label="文件内查找"
      title="文件内查找"
      onclick={() => view && openSearchPanel(view)}
      ><IconSearch aria-hidden="true" /></Button
    >
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label="跳转行"
      title="跳转行"
      onclick={() => view && openGotoLine(view)}
      ><IconListNumbers aria-hidden="true" /></Button
    >
    <Button
      variant={showGutter ? "secondary" : "ghost"}
      size="icon-sm"
      aria-label="显示行号"
      title={showGutter ? "隐藏行号" : "显示行号"}
      aria-pressed={showGutter}
      onclick={toggleGutter}><IconNumber aria-hidden="true" /></Button
    >
  </div>
  <div class="relative min-h-0 flex-1 overflow-hidden">
    <div
      bind:this={host}
      data-code-reader
      class="h-full overflow-hidden"
      aria-label="只读代码"
    ></div>
  </div>
</div>
