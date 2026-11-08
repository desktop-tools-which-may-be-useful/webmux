<script lang="ts">
  import type { Snippet } from "svelte";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
  import SessionViewControls from "./SessionViewControls.svelte";

  // The icon controls of the home page: list density, a Git refresh and the
  // repository menu. Phones show them in the top bar, wider screens in the
  // toolbar, so the title and these controls never take two rows.
  let {
    compact,
    refreshing,
    onchange,
    onrefresh,
    menu,
  }: {
    compact: boolean;
    refreshing: boolean;
    onchange: (compact: boolean) => void;
    onrefresh: () => void;
    // The "⋯" menu of the repository in scope; absent when none is.
    menu?: Snippet;
  } = $props();
</script>

<SessionViewControls {compact} {onchange} />
<Button
  variant="ghost"
  size="icon-sm"
  class="min-h-9 min-w-9 sm:min-h-8 sm:min-w-8"
  aria-label="刷新 Git 状态"
  title={refreshing ? "正在刷新 Git 状态…" : "刷新 Git 状态"}
  aria-busy={refreshing}
  onclick={() => onrefresh()}
  ><IconRefresh
    class={refreshing ? "animate-spin motion-reduce:animate-none" : ""}
    aria-hidden="true"
  /></Button
>
{#if menu}{@render menu()}{/if}
