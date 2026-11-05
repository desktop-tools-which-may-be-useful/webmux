<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Command from "$lib/components/ui/command";
  import * as Popover from "$lib/components/ui/popover";
  import { worktreeTitle } from "../homeWorktrees";
  import type { Worktree } from "../schemas";

  // Which of a repository's worktrees the home page lists. Every worktree is
  // one checkable row; a worktree with sessions stays listed whatever its
  // check, so the row says so instead of hiding it.
  let {
    repositoryName,
    worktrees,
    pinned,
    active,
    shown,
    ontoggle,
  }: {
    repositoryName: string;
    worktrees: readonly Worktree[];
    pinned: ReadonlySet<string>;
    /** Worktrees listed only because they have sessions or archived records. */
    active: ReadonlySet<string>;
    /** How many worktrees the home page lists. */
    shown: number;
    ontoggle: (worktreeId: string, pinned: boolean) => void;
  } = $props();
  let open = $state(false);
  let search = $state("");
  $effect(() => {
    if (!open) search = "";
  });
</script>

<Popover.Root bind:open>
  <Popover.Trigger>
    {#snippet child({ props })}<Button
        {...props}
        size="sm"
        variant="outline"
        class="min-h-9 sm:min-h-8"
        aria-label={`选择主页工作树 ${repositoryName}`}
        >主页显示 <span class="tabular-nums">{shown}/{worktrees.length}</span>
        <IconChevronDown class="size-4" aria-hidden="true" /></Button
      >{/snippet}
  </Popover.Trigger>
  <Popover.Content align="end" class="w-[min(28rem,calc(100vw-2rem))] p-0">
    <Command.Root
      label="主页工作树"
      class="h-auto min-w-0"
      filter={(value, term, keywords) =>
        [value, ...(keywords ?? [])].some((text) =>
          text.toLowerCase().includes(term.trim().toLowerCase()),
        )
          ? 1
          : 0}
      disableInitialScroll
    >
      <Command.Input
        bind:value={search}
        placeholder="按分支或路径搜索…"
        autocomplete="off"
        autocapitalize="off"
        autocorrect="off"
        spellcheck={false}
      />
      <Command.List
        aria-label="工作树列表"
        aria-multiselectable="true"
        class="max-h-72"
      >
        <Command.Empty>没有匹配的 worktree</Command.Empty>
        {#each worktrees as tree (tree.id)}
          <Command.Item
            value={tree.id}
            keywords={[worktreeTitle(tree), tree.path]}
            data-checked={pinned.has(tree.id)}
            aria-label={worktreeTitle(tree)}
            onSelect={() => ontoggle(tree.id, !pinned.has(tree.id))}
          >
            <span class="min-w-0 flex-1 space-y-0.5">
              <span class="flex flex-wrap items-center gap-1.5">
                <span class="font-medium break-all">{worktreeTitle(tree)}</span>
                {#if tree.kind === "main"}<Badge variant="secondary"
                    >主工作区</Badge
                  >{/if}
                {#if active.has(tree.id) && !pinned.has(tree.id)}<Badge
                    variant="outline">有会话，始终显示</Badge
                  >{/if}
                {#if !tree.available}<Badge variant="destructive">不可用</Badge
                  >{/if}
              </span>
              <span
                class="block font-mono text-xs break-all text-muted-foreground"
                >{tree.path}</span
              >
            </span>
          </Command.Item>
        {/each}
      </Command.List>
    </Command.Root>
    <p class="border-t px-3 py-2 text-xs text-muted-foreground">
      取消勾选只把它移出主页，不会删除 worktree。
    </p>
  </Popover.Content>
</Popover.Root>
