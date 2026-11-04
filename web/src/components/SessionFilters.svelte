<script lang="ts">
  import IconSearch from "~icons/tabler/search";
  import { Button } from "$lib/components/ui/button";
  import * as InputGroup from "$lib/components/ui/input-group";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import type { SessionInfo } from "../schemas";
  import { countSessions, type StatusFilter } from "../session-status";

  let {
    sessions,
    archivedCount,
    query = $bindable(),
    statusFilter = $bindable(),
    filtered,
    shortcutDisabled,
    onclear,
  }: {
    // Unfiltered list: the filter counts never depend on the active filter.
    sessions: SessionInfo[];
    // Archived hybrid records, also unfiltered.
    archivedCount: number;
    query: string;
    statusFilter: StatusFilter;
    // Filters are active and at least one session still matches.
    filtered: boolean;
    // A dialog or sheet owns the keyboard.
    shortcutDisabled: boolean;
    onclear: () => void;
  } = $props();

  let searchInput = $state<HTMLInputElement | null>(null);
  const filters = $derived.by(() => {
    const counts = countSessions(sessions);
    return [
      { id: "all", label: "全部", count: sessions.length },
      { id: "active", label: "运行中", count: counts.active },
      { id: "waiting", label: "等待中", count: counts.waiting },
      { id: "exited", label: "已退出", count: counts.exited },
      { id: "archived", label: "已归档", count: archivedCount },
    ] as const;
  });

  function searchShortcut(event: KeyboardEvent) {
    if (
      event.defaultPrevented ||
      event.isComposing ||
      event.metaKey ||
      event.ctrlKey ||
      event.altKey ||
      shortcutDisabled
    )
      return;
    const target = event.target;
    if (
      event.key !== "/" ||
      (target instanceof HTMLElement &&
        (target.isContentEditable ||
          target.closest('input, textarea, select, [role="dialog"]')))
    )
      return;
    event.preventDefault();
    searchInput?.focus();
  }
</script>

<svelte:window onkeydown={searchShortcut} />

<div class="flex flex-col gap-2 md:flex-row md:items-center md:gap-4">
  <InputGroup.Root class="min-w-0 md:flex-1">
    <InputGroup.Addon>
      <IconSearch aria-hidden="true" />
    </InputGroup.Addon>
    <InputGroup.Input
      type="search"
      aria-label="搜索会话"
      aria-keyshortcuts="/"
      title="按 / 搜索，Esc 清除筛选"
      placeholder="搜索会话、仓库、目录或分支"
      bind:ref={searchInput}
      bind:value={query}
      onkeydown={(event) => {
        if (event.key === "Escape" && !event.isComposing) {
          event.preventDefault();
          onclear();
        }
      }}
    />
    <InputGroup.Addon align="inline-end" class="hidden md:flex">
      <kbd class="border px-1" aria-hidden="true">/</kbd>
    </InputGroup.Addon>
  </InputGroup.Root>
  <SegmentedControl.Root
    value={statusFilter}
    onValueChange={(value) => {
      const next = filters.find((filter) => filter.id === value);
      if (next) statusFilter = next.id;
    }}
    size="form"
    aria-label="按会话状态筛选"
    wrap
  >
    {#each filters as filter (filter.id)}
      <SegmentedControl.Item value={filter.id} class="min-h-9">
        {filter.label}
        <span
          class="inline-block w-[3ch] font-mono text-xs text-right tabular-nums opacity-60"
          >{filter.count}</span
        >
      </SegmentedControl.Item>
    {/each}
  </SegmentedControl.Root>
</div>
{#if filtered}
  <div class="flex items-center gap-2 text-xs text-muted-foreground">
    <span>已筛选会话</span>
    <Button
      variant="link"
      size="sm"
      class="min-h-9 sm:min-h-8"
      onclick={onclear}>清除筛选</Button
    >
  </div>
{/if}
