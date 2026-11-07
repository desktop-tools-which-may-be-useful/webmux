<script lang="ts">
  import type { ToolEntry } from "../../chat-model";
  import type { ChatCodeLink } from "../../markdown-link";
  import ChatDisclosure from "./ChatDisclosure.svelte";
  import ToolDetails from "./ToolDetails.svelte";
  import ToolRow from "./ToolRow.svelte";
  import ToolSummary from "./ToolSummary.svelte";

  // A run of consecutive tool calls folded into one row. The collapsed row
  // shows the latest call, so its spinner is the live progress indicator; a
  // call that waits for permission takes its place so it is never hidden.
  // Opening the row lists every call, each with its own details. A group of
  // one opens straight to that call's details.
  let {
    calls,
    waiting,
    onreview,
    onopenlocation,
  }: {
    calls: readonly ToolEntry[];
    /** Tool calls whose permission request waits for an answer. */
    waiting: readonly string[];
    /** Opens a changed file in the code panel. */
    onreview: (path: string) => void;
    /** Opens a referenced code file or line in the code panel. */
    onopenlocation?: (target: ChatCodeLink) => void;
  } = $props();

  const shown = $derived(
    calls.find((call) => waiting.includes(call.id)) ?? calls.at(-1),
  );
</script>

{#if shown}
  <div
    class="min-w-0"
    data-tool-call={calls.length === 1 ? shown.id : undefined}
    data-tool-group={calls[0]?.id}
  >
    <ChatDisclosure>
      {#snippet summary()}
        <ToolSummary tool={shown} waiting={waiting.includes(shown.id)} />
        {#if calls.length > 1}
          <span class="shrink-0 text-xs tabular-nums @max-xs:hidden"
            >共 {calls.length} 步</span
          >
        {/if}
      {/snippet}
      {#if calls.length === 1}
        <ToolDetails tool={shown} {onreview} {onopenlocation} />
      {:else}
        {#each calls as call (call.id)}
          <ToolRow
            tool={call}
            waiting={waiting.includes(call.id)}
            {onreview}
            {onopenlocation}
          />
        {/each}
      {/if}
    </ChatDisclosure>
  </div>
{/if}
