<script lang="ts">
  import type { Snippet } from "svelte";
  import { Button } from "$lib/components/ui/button";

  let {
    title = "工作区",
    actions,
    loading,
    filtering,
    visibleCount,
    sessionCount,
    repositoryCount,
    refreshing,
    onstart,
    children,
  }: {
    title?: string;
    /** Icon controls; phones show them in the top bar instead. */
    actions: Snippet;
    loading: boolean;
    filtering: boolean;
    visibleCount: number;
    sessionCount: number;
    repositoryCount: number;
    refreshing: boolean;
    // Opens the new-session dialog; the old header's primary action.
    onstart: () => void;
    // Search and status filters below the title row.
    children: Snippet;
  } = $props();
</script>

<div class="mb-5 space-y-3 border-b pb-4">
  <div
    class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 sm:gap-x-1"
  >
    <p role="status" class="sr-only">
      {#if loading}正在加载…
      {:else if filtering}显示 {visibleCount} / {sessionCount} 个会话
      {:else}{repositoryCount} 个仓库 · {sessionCount} 个会话{/if}{#if refreshing}
        · 正在读取 Git 状态…{/if}
    </p>
    <!-- Phones already show the title in the top bar. -->
    <h1
      class="hidden min-w-0 text-lg font-semibold tracking-tight break-all sm:mr-3 sm:block"
    >
      {title}
    </h1>
    <div
      class="ml-auto hidden max-w-full items-center justify-end gap-1 sm:flex"
    >
      {@render actions()}
    </div>
    <Button
      size="sm"
      class="order-last min-h-9 w-full sm:order-none sm:min-h-8 sm:w-auto"
      onclick={() => onstart()}>＋ 新建会话</Button
    >
  </div>
  {@render children()}
</div>
