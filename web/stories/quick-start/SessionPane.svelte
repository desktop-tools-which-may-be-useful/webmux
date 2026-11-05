<script lang="ts">
  import HybridSession from "../acp/HybridSession.svelte";
  import type { Agent } from "../acp/fixtures";
  import type { Entry } from "./data";

  // The session page in the shell's main area, using the hybrid session
  // story: its chat view on the chat side, its terminal otherwise. Ordinary
  // terminal sessions reuse the terminal scenario, so their header still
  // shows the hybrid view tabs; the real page shows the terminal header.
  let { entry }: { entry: Entry } = $props();
  const agents: Record<string, Agent> = {
    claudecode: "claudecode",
    pi: "omp",
    opencode: "opencode",
  };
  const agent = $derived(agents[entry.icon] ?? "omp");
</script>

{#key entry.id}
  <HybridSession
    scenario={entry.side === "acp" ? "conversation" : "terminal"}
    {agent}
  />
{/key}
