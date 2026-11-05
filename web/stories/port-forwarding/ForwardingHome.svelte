<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import ForwardDialog from "../../src/components/ForwardDialog.svelte";
  import SidebarFooter from "../../src/components/SidebarFooter.svelte";
  import { demoForwards, demoSessions, serveForwards } from "./demo";

  // The sidebar footer with its port-forwarding entry, and the dialog it
  // opens, over demo forwards answered in memory.
  let {
    dialog = true,
    empty = false,
    detection = true,
  }: { dialog?: boolean; empty?: boolean; detection?: boolean } = $props();
  let open = $state(untrack(() => dialog));
  onDestroy(serveForwards(untrack(() => (empty ? [] : demoForwards))));
</script>

<div class="flex min-h-dvh">
  <aside class="flex w-72 flex-col justify-end border-r bg-card">
    <SidebarFooter
      connection="connected"
      hostname="devbox"
      setupSettled
      onforward={() => (open = true)}
    />
  </aside>
  <main class="flex-1 px-8 py-6 text-sm text-muted-foreground">
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
