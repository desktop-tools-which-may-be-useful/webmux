<script lang="ts">
  import type { Snippet } from "svelte";
  import IconFolderPlus from "~icons/tabler/folder-plus";
  import IconSettings from "~icons/tabler/settings";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";

  let {
    title = "工作区",
    controls,
    loading,
    filtering,
    visibleCount,
    sessionCount,
    repositoryCount,
    canAddRepository,
    refreshing,
    onadd,
    onsettings,
    onrefresh,
    onstart,
    children,
  }: {
    title?: string;
    controls?: Snippet;
    loading: boolean;
    filtering: boolean;
    visibleCount: number;
    sessionCount: number;
    repositoryCount: number;
    canAddRepository: boolean;
    refreshing: boolean;
    onadd: () => void;
    // Opens the settings of the repository in scope; absent when none is.
    onsettings?: () => void;
    onrefresh: () => void;
    // Opens the new-session dialog; the old header's primary action.
    onstart: () => void;
    // Search and status filters below the title row.
    children: Snippet;
  } = $props();
</script>

<div class="mb-5 space-y-3 border-b pb-4">
  <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
    <div class="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
      <h1 class="text-lg font-semibold tracking-tight break-all">{title}</h1>
      <p role="status" class="sr-only">
        {#if loading}正在加载…
        {:else if filtering}显示 {visibleCount} / {sessionCount} 个会话
        {:else}{repositoryCount} 个仓库 · {sessionCount} 个会话{/if}{#if refreshing}
          · 正在读取 Git 状态…{/if}
      </p>
    </div>
    <div
      class="ml-auto flex max-w-full flex-wrap items-center justify-end gap-1"
    >
      {@render controls?.()}
      <Button
        variant="ghost"
        size="sm"
        class="min-h-9 sm:min-h-8"
        disabled={!canAddRepository}
        onclick={() => onadd()}
        ><IconFolderPlus aria-hidden="true" />添加仓库</Button
      >
      {#if onsettings}<Button
          variant="ghost"
          size="sm"
          class="min-h-9 sm:min-h-8"
          onclick={() => onsettings()}
          ><IconSettings aria-hidden="true" />仓库设置</Button
        >{/if}
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
      <Button size="sm" class="min-h-9 sm:min-h-8" onclick={() => onstart()}
        >＋ 新建会话</Button
      >
    </div>
  </div>
  {@render children()}
</div>
