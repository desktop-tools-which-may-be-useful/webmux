<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import ForwardDialog from "../../src/components/ForwardDialog.svelte";
  import WorkspaceHeader from "../../src/components/WorkspaceHeader.svelte";
  import { demoForwards, demoSessions, serveForwards } from "./demo";

  // The home page header with its port-forwarding entry, and the dialog it
  // opens, over demo forwards answered in memory.
  let {
    dialog = true,
    empty = false,
    detection = true,
  }: { dialog?: boolean; empty?: boolean; detection?: boolean } = $props();
  let open = $state(untrack(() => dialog));
  onDestroy(serveForwards(untrack(() => (empty ? [] : demoForwards))));
</script>

<div class="workspace-page min-h-dvh">
  <WorkspaceHeader
    connection="connected"
    hostname="devbox"
    setupSettled
    canStart
    onstart={() => {}}
    onforward={() => (open = true)}
  />
  <main
    class="mx-auto max-w-7xl px-4 py-6 text-sm text-muted-foreground sm:px-8"
  >
    会话列表（示例省略）
  </main>
</div>

{#if open}
  <ForwardDialog
    sessions={demoSessions}
    {detection}
    onclose={() => (open = false)}
  />
{/if}
