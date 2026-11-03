<script lang="ts">
  import type { Connection } from "../../src/ws";
  import TerminalViewport from "../../src/components/TerminalViewport.svelte";

  let {
    connection = "reconnecting" as Connection,
    restoring = false,
    atBottom = true,
  }: {
    connection?: Connection;
    restoring?: boolean;
    atBottom?: boolean;
  } = $props();

  const terminal = {
    selection: "",
    get restoring() {
      return restoring;
    },
    get atBottom() {
      return atBottom;
    },
    get connection() {
      return connection;
    },
    modifiers: { ctrl: false, alt: false, shift: false },
    shiftLocked: false,
    copySelection() {},
    clearSelection() {},
    scrollToBottom() {},
  };
  const uploads = { dragging: false, progress: null };
</script>

{#snippet quickMenu()}<span></span>{/snippet}
{#snippet draftToggle()}<span></span>{/snippet}

<div style="position: relative; width: 400px; height: 200px">
  <TerminalViewport
    {terminal}
    visible
    {uploads}
    {quickMenu}
    {draftToggle}
    send={() => {}}
    toggle={() => {}}
    lock={() => {}}
    paste={async () => {}}
    keyboard={() => {}}
  />
</div>
