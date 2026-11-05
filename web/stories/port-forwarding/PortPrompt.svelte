<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import ForwardDialog from "../../src/components/ForwardDialog.svelte";
  import TerminalNotices from "../../src/components/TerminalNotices.svelte";
  import { createTerminalDrafts } from "../../src/terminal-draft.svelte";
  import { demoSession, serveForwards } from "./demo";

  // The terminal page's port prompts for session "web", over demo forwards
  // answered in memory. `failing` makes every new forward fail.
  let {
    multiple = false,
    failing = false,
  }: { multiple?: boolean; failing?: boolean } = $props();
  const web = demoSession(
    "webwebwe",
    "web",
    untrack(() => (multiple ? [3000, 5173, 6006, 9229] : [5173])).map(
      (port) => ({ port, process: "node" }),
    ),
  );
  onDestroy(
    serveForwards(
      [],
      untrack(() => (failing ? "port already forwarded: 5173" : "")),
    ),
  );
  let dialog = $state(false);
  let notice = $state("");
</script>

<div class="flex h-dvh flex-col bg-background">
  <div
    class="flex h-8 shrink-0 items-center gap-2 border-b bg-card px-3 text-sm"
  >
    <span class="status-dot status-running" aria-hidden="true"></span>
    <span class="min-w-0 truncate font-medium">{web.title}</span>
    <span class="truncate text-xs text-muted-foreground">{web.cwd}</span>
  </div>
  <div class="relative min-h-0 flex-1">
    <div class="absolute inset-x-0 top-0 z-30 flex flex-col bg-background">
      <TerminalNotices
        terminal={{
          control: undefined,
          controlling: true,
          connection: "connected",
          exitCode: null,
          error: "",
          claim: () => {},
        }}
        uploads={{ progress: null, cancel: () => {} }}
        draft={createTerminalDrafts().get(web.id)}
        draftSaved=""
        error=""
        bind:notice
        showScratch={() => {}}
        dismissScratch={() => {}}
        remove={() => {}}
        session={web.id}
        listening={web.listeningPorts}
        showForwards={() => (dialog = true)}
      />
    </div>
    <pre
      class="h-full overflow-auto p-3 font-mono text-xs leading-5">$ pnpm dev --base /forward/5173/

  VITE v6.2.0  ready in 412 ms

  ➜  Local:   http://localhost:5173/forward/5173/
  ➜  Network: use --host to expose
  ➜  press h + enter to show help</pre>
  </div>
</div>

{#if dialog}
  <ForwardDialog sessions={[web]} detection onclose={() => (dialog = false)} />
{/if}
