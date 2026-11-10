<script lang="ts">
  import { MAX_BATCH_WORKTREES } from "../schemas";
  import type { WorktreeBatchEntry } from "../worktree-batch";
  import { Badge } from "$lib/components/ui/badge";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";

  let {
    entries,
    filteredEntries,
    selectedIds,
    busy,
    ontoggle,
  }: {
    entries: readonly WorktreeBatchEntry[];
    filteredEntries: readonly WorktreeBatchEntry[];
    selectedIds: readonly string[];
    busy: boolean;
    ontoggle: (id: string, checked: boolean) => void;
  } = $props();

  const selectedSet = $derived(new Set(selectedIds));
</script>

{#if filteredEntries.length === 0}
  <p
    class="rounded-lg border bg-muted/30 px-4 py-6 text-center text-sm text-muted-foreground"
  >
    {entries.length === 0
      ? "该仓库暂无工作树。"
      : "没有匹配当前筛选条件的工作树。"}
  </p>
{:else}
  <ul
    class="max-h-[45dvh] space-y-2 overflow-y-auto pr-1"
    aria-label="仓库工作树列表"
  >
    {#each filteredEntries as entry (entry.tree.id)}
      {@const isProtected = entry.category === "protected"}
      {@const isChecked = !isProtected && selectedSet.has(entry.tree.id)}
      <li
        class={["rounded-lg border p-3 text-sm", isProtected && "bg-muted/30"]}
      >
        <Label class="w-full items-start">
          <Checkbox
            checked={isChecked}
            disabled={isProtected ||
              busy ||
              (!isChecked && selectedIds.length >= MAX_BATCH_WORKTREES)}
            aria-label={entry.title}
            onCheckedChange={(value) => ontoggle(entry.tree.id, value === true)}
            class="mt-0.5"
          />
          <div class="min-w-0 flex-1 space-y-1">
            <div class="flex flex-wrap items-center gap-1.5">
              <span class="font-medium break-all">{entry.title}</span>
              {#if entry.tree.head}
                <span class="font-mono text-xs text-muted-foreground"
                  >{entry.tree.head.slice(0, 8)}</span
                >
              {/if}
              {#if entry.tree.kind === "main"}
                <Badge variant="secondary">主工作区</Badge>
              {:else if entry.tree.kind === "bare"}
                <Badge variant="secondary">bare</Badge>
              {/if}
              {#if !entry.pinned && entry.tree.kind === "linked"}
                <Badge variant="outline">未添加到主页</Badge>
              {/if}
              {#if entry.tree.locked}
                <Badge variant="outline">已锁定</Badge>
              {/if}
              {#if entry.tree.prunable && !entry.tree.available}
                <Badge variant="outline">Git 登记待清理</Badge>
              {:else if !entry.tree.available}
                <Badge variant="destructive">不可用</Badge>
              {:else if entry.category === "safe"}
                <Badge variant="secondary">可安全清理</Badge>
              {/if}
              {#if entry.tree.dirty === true}
                <Badge variant="warning">有修改</Badge>
              {/if}
              {#if entry.tree.hasUntracked}
                <Badge variant="outline">未跟踪文件</Badge>
              {/if}
              {#if entry.tree.hasIgnored}
                <Badge variant="outline">含忽略文件</Badge>
              {/if}
              {#if entry.activeSessionCount > 0}
                <Badge variant="warning"
                  >{entry.activeSessionCount} 个运行中会话</Badge
                >
              {/if}
              {#if entry.archivedCount > 0}
                <Badge variant="secondary"
                  >{entry.archivedCount} 条归档聊天</Badge
                >
              {/if}
            </div>
            <p class="font-mono text-xs break-all text-muted-foreground">
              {entry.tree.path}
            </p>
            {#if isProtected}
              <p class="text-xs break-all text-muted-foreground">
                {entry.reason}
              </p>
            {/if}
          </div>
        </Label>
      </li>
    {/each}
  </ul>
{/if}
