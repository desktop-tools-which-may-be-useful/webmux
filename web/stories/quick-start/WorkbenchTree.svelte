<script lang="ts">
  import { SvelteSet } from "svelte/reactivity";
  import IconAlertCircle from "~icons/tabler/alert-circle";
  import IconBook2 from "~icons/tabler/book-2";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconFolder from "~icons/tabler/folder";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconLayoutDashboard from "~icons/tabler/layout-dashboard";
  import IconPencil from "~icons/tabler/pencil";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import LaunchConfigIcon from "../../src/components/LaunchConfigIcon.svelte";
  import {
    directEntries,
    repositories,
    statusLabels,
    type Entry,
  } from "./data";
  import StatusText from "./StatusText.svelte";
  import { titleOf } from "./titles.svelte";

  // Prototype of the workbench-mode sidebar content: repositories, their
  // worktrees and the sessions running in each. Collapsed nodes keep their
  // session and waiting counts; each available repository has a "+" that
  // opens the existing new-worktree flow, each session a rename button for
  // the existing dialog. Arrow-key navigation (roving tabindex) is left to
  // the implementation (design.md §9).
  let {
    scope,
    current,
    onscope,
    onopen,
    onnewworktree,
    onrename,
  }: {
    scope: string;
    current: string | null;
    onscope: (scope: string, worktree?: string) => void;
    onopen: (id: string) => void;
    onnewworktree: (repositoryId: string) => void;
    onrename: (id: string) => void;
  } = $props();
  const expanded = new SvelteSet<string>(["webmux", "wt-brisk", "docs-site"]);
  function toggle(id: string) {
    if (expanded.has(id)) expanded.delete(id);
    else expanded.add(id);
  }
</script>

{#snippet chevron(id: string, label: string)}
  <Button
    variant="quiet"
    size="icon-xs"
    aria-label={`${expanded.has(id) ? "收起" : "展开"}${label}`}
    title={`${expanded.has(id) ? "收起" : "展开"}${label}`}
    onclick={() => toggle(id)}
    ><IconChevronRight
      class={`transition-transform motion-reduce:transition-none ${expanded.has(id) ? "rotate-90" : ""}`}
      aria-hidden="true"
    /></Button
  >
{/snippet}

{#snippet counts(entries: Entry[])}
  {@const wait = entries.filter((item) => item.status === "waiting").length}
  {#if wait}<span class="shrink-0 text-xs text-warning">{wait} 等待</span
    >{:else if entries.length}<span
      class="shrink-0 text-xs text-muted-foreground">{entries.length}</span
    >{/if}
{/snippet}

{#snippet session(item: Entry, level: number)}
  {@const title = titleOf(item)}
  <li
    role="treeitem"
    aria-level={level}
    aria-selected={item.id === current}
    class="group/row relative"
  >
    <ListButton
      variant="file-tree"
      class={level === 3 ? "pl-12" : "pl-7"}
      aria-current={item.id === current ? "page" : undefined}
      title={`${title}（${statusLabels[item.status]}）`}
      onclick={() => onopen(item.id)}
    >
      <LaunchConfigIcon icon={item.icon} size="xs" />
      <span
        class={`min-w-0 flex-1 truncate ${item.status === "ended" ? "text-muted-foreground" : ""}`}
        >{title}</span
      >
      <StatusText status={item.status} />
    </ListButton>
    <ListButton
      variant="file-tree-action"
      aria-label={`修改标题：${title}`}
      title={`修改标题：${title}`}
      onclick={() => onrename(item.id)}
      ><IconPencil aria-hidden="true" /></ListButton
    >
  </li>
{/snippet}

<ul role="tree" aria-label="仓库与会话" class="space-y-0.5 text-sm">
  <li role="treeitem" aria-level={1} aria-selected={scope === "all"}>
    <ListButton
      variant="file-tree"
      class="pl-7"
      aria-current={scope === "all" && !current ? "page" : undefined}
      onclick={() => onscope("all")}
      ><IconLayoutDashboard aria-hidden="true" />全部仓库</ListButton
    >
  </li>
  {#each repositories as repo (repo.id)}
    {@const entries = repo.worktrees.flatMap((tree) => tree.entries)}
    <li
      role="treeitem"
      aria-level={1}
      aria-expanded={repo.reason ? undefined : expanded.has(repo.id)}
      aria-selected={scope === repo.id}
    >
      <div class="flex items-center">
        {#if repo.reason}<span class="size-6 shrink-0"
          ></span>{:else}{@render chevron(repo.id, repo.name)}{/if}
        <div class="group/row relative min-w-0 flex-1">
          <ListButton
            variant="file-tree"
            aria-current={scope === repo.id && !current ? "page" : undefined}
            title={repo.reason ?? repo.name}
            onclick={() => onscope(repo.id)}
          >
            <IconBook2 aria-hidden="true" />
            <span
              class={`min-w-0 flex-1 truncate font-medium ${repo.reason ? "text-muted-foreground" : ""}`}
              >{repo.name}</span
            >
            {#if repo.reason}<IconAlertCircle
                class="text-warning"
                aria-label="不可用"
              />{:else}{@render counts(entries)}{/if}
          </ListButton>
          {#if !repo.reason}
            <ListButton
              variant="file-tree-action"
              aria-label={`在 ${repo.name} 新建工作树`}
              title={`在 ${repo.name} 新建工作树`}
              onclick={() => onnewworktree(repo.id)}
              ><IconPlus aria-hidden="true" /></ListButton
            >
          {/if}
        </div>
      </div>
      {#if expanded.has(repo.id) && !repo.reason}
        <ul role="group" class="space-y-0.5">
          {#each repo.worktrees as tree (tree.id)}
            <li
              role="treeitem"
              aria-level={2}
              aria-expanded={expanded.has(tree.id)}
              aria-selected={false}
            >
              <div class="flex items-center pl-4">
                {@render chevron(tree.id, tree.title)}
                <ListButton
                  variant="file-tree"
                  class="min-w-0 flex-1"
                  title={tree.path}
                  onclick={() => onscope(repo.id, tree.id)}
                >
                  <IconGitBranch aria-hidden="true" />
                  <span class="min-w-0 flex-1 truncate">{tree.title}</span>
                  {@render counts(tree.entries)}
                </ListButton>
              </div>
              {#if expanded.has(tree.id)}
                <ul role="group" class="space-y-0.5">
                  {#each tree.entries as item (item.id)}
                    {@render session(item, 3)}
                  {/each}
                </ul>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </li>
  {/each}
  <li
    role="treeitem"
    aria-level={1}
    aria-expanded={expanded.has("direct")}
    aria-selected={scope === "direct"}
  >
    <div class="flex items-center">
      {@render chevron("direct", "直接会话")}
      <ListButton
        variant="file-tree"
        class="min-w-0 flex-1"
        aria-current={scope === "direct" && !current ? "page" : undefined}
        onclick={() => onscope("direct")}
      >
        <IconFolder aria-hidden="true" />
        <span class="min-w-0 flex-1 truncate">直接会话</span>
        {@render counts(directEntries)}
      </ListButton>
    </div>
    {#if expanded.has("direct")}
      <ul role="group" class="space-y-0.5">
        {#each directEntries as item (item.id)}
          {@render session(item, 2)}
        {/each}
      </ul>
    {/if}
  </li>
</ul>
