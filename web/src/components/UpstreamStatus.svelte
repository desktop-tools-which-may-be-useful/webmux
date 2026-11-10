<script lang="ts">
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import { Badge } from "$lib/components/ui/badge";
  import {
    aheadLabel,
    behindLabel,
    upstreamText,
    type UpstreamState,
  } from "../worktree-sync";

  // A branch's or worktree's upstream: its name, then a coloured arrow and
  // the count for the commits ahead (green, outgoing) and behind (blue,
  // incoming), or the words for a state without counts. `indicator` keeps
  // only what needs attention (counts, a deleted upstream). Items flow into
  // the parent's flex row.
  let {
    state,
    indicator = false,
  }: {
    state: UpstreamState | { kind: "unknown" };
    indicator?: boolean;
  } = $props();
  const text = $derived(upstreamText(state));
</script>

{#snippet count(
  Icon: typeof IconArrowUp,
  color: string,
  n: number,
  label: string,
)}
  <span
    class="inline-flex shrink-0 items-center text-xs font-medium tabular-nums"
    role="img"
    aria-label={label}
    title={label}
    ><Icon class={`size-3.5 ${color}`} aria-hidden="true" />{n}</span
  >
{/snippet}

{#if !indicator && "upstream" in state}<span class="min-w-0 font-mono break-all"
    >{state.upstream}</span
  >{/if}
{#if state.kind === "tracked"}
  {#if state.ahead}{@render count(
      IconArrowUp,
      "text-success",
      state.ahead,
      aheadLabel(state.ahead),
    )}{/if}
  {#if state.behind}{@render count(
      IconArrowDown,
      "text-info",
      state.behind,
      behindLabel(state.behind),
    )}{/if}
{:else if state.kind === "gone"}<Badge variant="warning">{text}</Badge>{/if}
{#if !indicator && state.kind !== "gone" && text}<span
    class="text-muted-foreground">{text}</span
  >{/if}
