<script lang="ts">
  import type { SearchSpec } from "../code-search";
  import CodeSearchPanel from "./CodeSearchPanel.svelte";
  import CodeGotoLine from "./CodeGotoLine.svelte";

  let {
    panel,
    query,
    line,
    status,
    onquery,
    onnext,
    onprevious,
    ongoto,
    onclose,
  }: {
    panel?: "search" | "goto";
    query: SearchSpec;
    line: number;
    status?: string;
    onquery: (query: SearchSpec) => void;
    onnext: () => void;
    onprevious: () => void;
    ongoto: (value: string) => void;
    onclose: () => void;
  } = $props();
</script>

{#if panel === "search"}
  <div class="shrink-0 border-b">
    <CodeSearchPanel {query} {onquery} {onnext} {onprevious} {onclose} />
    {#if status}
      <p class="px-2 pb-1 text-xs text-muted-foreground" role="status">
        {status}
      </p>
    {/if}
  </div>
{:else if panel === "goto"}
  <div class="shrink-0 border-b">
    <CodeGotoLine {line} onsubmit={ongoto} {onclose} />
  </div>
{/if}
