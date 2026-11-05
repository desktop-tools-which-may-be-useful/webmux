<script lang="ts" module>
  import { z } from "zod";
  import { homeLayout, worktreeTitle } from "../homeWorktrees";
  import type { HomeWorktrees, Repository, SessionInfo } from "../schemas";
  import { rowStatus } from "../session-status";

  export type TreeNode = {
    key: string;
    level: number;
    parent?: string;
    kind: "all" | "repository" | "direct" | "worktree" | "missing" | "session";
    label: string;
    /** Repository or worktree ID; the session for session rows. */
    repositoryId?: string;
    worktreeId?: string;
    session?: SessionInfo;
    path?: string;
    /** Why a repository is unavailable; "" when it is fine. */
    reason?: string;
    children: TreeNode[];
    /** Live sessions below the node, and how many of them wait. */
    count: number;
    waiting: number;
  };

  // Newest first by creation, as the home cards: output never reorders rows.
  const byCreation = (a: SessionInfo, b: SessionInfo) =>
    Date.parse(b.createdAt) - Date.parse(a.createdAt);

  function sessionNodes(
    sessions: readonly SessionInfo[],
    parent: string,
    level: number,
  ): TreeNode[] {
    return [...sessions].sort(byCreation).map((session) => ({
      key: `s:${session.id}`,
      level,
      parent,
      kind: "session",
      label: session.title,
      session,
      children: [],
      count: 0,
      waiting: 0,
    }));
  }

  function counted(
    node: Omit<TreeNode, "count" | "waiting">,
    sessions: readonly SessionInfo[],
  ): TreeNode {
    return {
      ...node,
      count: sessions.length,
      waiting: sessions.filter((s) => rowStatus(s) === "waiting").length,
    };
  }

  /**
   * The workbench tree: all repositories, each registered repository with
   * the worktrees its home group shows and their live sessions, then direct
   * sessions. Worktree order is the home page's, independent of output.
   */
  export function workbenchTree(
    repositories: readonly Repository[],
    sessions: readonly SessionInfo[],
    home: HomeWorktrees,
  ): TreeNode[] {
    const nodes: TreeNode[] = [
      counted(
        { key: "all", level: 1, kind: "all", label: "全部仓库", children: [] },
        sessions,
      ),
    ];
    for (const repo of repositories) {
      const key = `r:${repo.id}`;
      const own = sessions.filter((s) => s.workspace?.repositoryId === repo.id);
      const reason =
        repo.availability === "available" ? "" : repo.error || "仓库当前不可用";
      const children: TreeNode[] = [];
      if (!reason) {
        for (const { tree } of homeLayout(home, repo, own).shown) {
          const treeKey = `w:${tree.id}`;
          const inside = own.filter((s) => s.workspace?.worktreeId === tree.id);
          children.push(
            counted(
              {
                key: treeKey,
                level: 2,
                parent: key,
                kind: "worktree",
                label: worktreeTitle(tree),
                repositoryId: repo.id,
                worktreeId: tree.id,
                path: tree.path,
                children: sessionNodes(inside, treeKey, 3),
              },
              inside,
            ),
          );
        }
        const missing = own.filter(
          (s) => !repo.worktrees.some((t) => t.id === s.workspace?.worktreeId),
        );
        if (missing.length)
          children.push(
            counted(
              {
                key: `m:${repo.id}`,
                level: 2,
                parent: key,
                kind: "missing",
                label: "已不可用工作区",
                repositoryId: repo.id,
                children: sessionNodes(missing, `m:${repo.id}`, 3),
              },
              missing,
            ),
          );
      }
      nodes.push(
        counted(
          {
            key,
            level: 1,
            kind: "repository",
            label: repo.name,
            repositoryId: repo.id,
            path: repo.anchorPath,
            reason,
            children,
          },
          own,
        ),
      );
    }
    const direct = sessions.filter(
      (s) =>
        !s.workspace ||
        !repositories.some((r) => r.id === s.workspace?.repositoryId),
    );
    nodes.push(
      counted(
        {
          key: "direct",
          level: 1,
          kind: "direct",
          label: "直接会话",
          children: sessionNodes(direct, "direct", 2),
        },
        direct,
      ),
    );
    return nodes;
  }

  const ExpandedSchema = z.record(z.string(), z.boolean());
</script>

<script lang="ts">
  import { tick } from "svelte";
  import IconAlertCircle from "~icons/tabler/alert-circle";
  import IconBook2 from "~icons/tabler/book-2";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconFolder from "~icons/tabler/folder";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconLayoutDashboard from "~icons/tabler/layout-dashboard";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { store, stored } from "../api";
  import { rowStatusLabels } from "../session-status";
  import LaunchConfigIcon from "./LaunchConfigIcon.svelte";
  import RowStatus from "./RowStatus.svelte";
  import SidebarSessionMenu from "./SidebarSessionMenu.svelte";

  // The workbench-mode sidebar: repositories → worktrees → live sessions.
  // One roving tab stop for the rows (arrow keys move, left and right fold),
  // and the focused row's action button is the next stop. Collapsed nodes
  // keep their counts and waiting numbers.
  let {
    repositories,
    sessions,
    home,
    scope,
    worktree,
    current,
    canCreate,
    configIcons,
    onscope,
    onopen,
    onnewworktree,
    onrename,
    onclose,
  }: {
    repositories: readonly Repository[];
    /** Live sessions, as the home page lists them. */
    sessions: readonly SessionInfo[];
    home: HomeWorktrees;
    /** The home view shown ("all", "direct" or a repository ID); "" elsewhere. */
    scope: string;
    /** The worktree the home view narrows to; "" for none. */
    worktree: string;
    /** The session page shown; "" elsewhere. */
    current: string;
    /** Git and the registry can create worktrees. */
    canCreate: boolean;
    /** Launch config icons by ID, for ordinary sessions. */
    configIcons: ReadonlyMap<string, string>;
    onscope: (scope: string, worktreeId?: string) => void;
    onopen: (session: SessionInfo) => void;
    onnewworktree: (repositoryId: string) => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
  } = $props();

  const nodes = $derived(workbenchTree(repositories, sessions, home));
  let saved = $state<Record<string, boolean>>(
    (() => {
      try {
        return ExpandedSchema.parse(JSON.parse(stored("workbenchTree", "{}")));
      } catch {
        return {};
      }
    })(),
  );
  // The current session's repository and worktree open without being saved.
  const forced = $derived.by(() => {
    // The keys of the current session's ancestors.
    const find = (list: TreeNode[], path: string[]): string[] => {
      for (const node of list) {
        if (current && node.session?.id === current) return path;
        const found = find(node.children, [...path, node.key]);
        if (found.length) return found;
      }
      return [];
    };
    return find(nodes, []);
  });
  function isOpen(node: TreeNode) {
    if (!node.children.length || node.reason) return false;
    if (forced.includes(node.key)) return true;
    // New repositories start open; worktrees and direct sessions, which list
    // sessions themselves, start closed.
    return saved[node.key] ?? node.kind === "repository";
  }
  function setOpen(node: TreeNode, open: boolean) {
    saved = { ...saved, [node.key]: open };
    store("workbenchTree", JSON.stringify(saved));
  }
  const visible = $derived.by(() => {
    const rows: TreeNode[] = [];
    const walk = (list: TreeNode[]) => {
      for (const node of list) {
        rows.push(node);
        if (isOpen(node)) walk(node.children);
      }
    };
    walk(nodes);
    return rows;
  });
  let focusKey = $state("all");
  const tabStop = $derived(
    visible.some((node) => node.key === focusKey)
      ? focusKey
      : (visible.find((node) => isCurrent(node))?.key ?? "all"),
  );
  let root = $state<HTMLElement>();
  async function focusRow(key: string) {
    focusKey = key;
    await tick();
    root
      ?.querySelector<HTMLElement>(`[data-tree-key="${CSS.escape(key)}"]`)
      ?.focus();
  }
  function keydown(event: KeyboardEvent, node: TreeNode) {
    const index = visible.findIndex((row) => row.key === node.key);
    let next: TreeNode | undefined;
    switch (event.key) {
      case "ArrowDown":
        next = visible[index + 1];
        break;
      case "ArrowUp":
        next = visible[index - 1];
        break;
      case "Home":
        next = visible[0];
        break;
      case "End":
        next = visible.at(-1);
        break;
      case "ArrowRight":
        if (node.children.length && !node.reason) {
          if (!isOpen(node)) setOpen(node, true);
          else next = node.children[0];
        }
        break;
      case "ArrowLeft":
        if (isOpen(node)) setOpen(node, false);
        else next = visible.find((row) => row.key === node.parent);
        break;
      default:
        return;
    }
    event.preventDefault();
    if (next) void focusRow(next.key);
  }
  function isCurrent(node: TreeNode) {
    if (node.kind === "session") return node.session?.id === current;
    if (current || node.kind === "missing") return false;
    if (node.kind === "worktree")
      return scope === node.repositoryId && worktree === node.worktreeId;
    return node.kind === "repository"
      ? scope === node.repositoryId && !worktree
      : scope === node.key;
  }
  function activate(node: TreeNode) {
    focusKey = node.key;
    if (node.session) onopen(node.session);
    else if (node.kind === "worktree" || node.kind === "missing")
      onscope(node.repositoryId ?? "all", node.worktreeId);
    else
      onscope(
        node.kind === "repository" ? (node.repositoryId ?? "all") : node.key,
      );
  }
  function sessionIcon(session: SessionInfo) {
    return (
      session.hybrid?.agentIcon ??
      (session.configId ? configIcons.get(session.configId) : undefined) ??
      ""
    );
  }
</script>

{#snippet counts(node: TreeNode)}
  <!-- Right-aligned, so counts end where session rows' status dots do. -->
  {#if node.waiting}<span
      class="w-12 shrink-0 truncate text-right text-xs text-warning"
      title={`${node.waiting} 个会话在等待`}>{node.waiting} 等待</span
    >{:else if node.count}<span
      class="w-12 shrink-0 text-right text-xs text-muted-foreground tabular-nums"
      >{node.count}</span
    >{/if}
{/snippet}

{#snippet chevron(node: TreeNode)}
  {#if node.children.length && !node.reason}
    {@const label = `${isOpen(node) ? "收起" : "展开"}${node.label}`}
    <!-- Keyboard users fold with the arrow keys, so this is not a tab stop. -->
    <Button
      variant="quiet"
      size="icon-xs"
      class="shrink-0"
      tabindex={-1}
      aria-label={label}
      title={label}
      onclick={() => setOpen(node, !isOpen(node))}
      ><IconChevronRight
        class={`transition-transform motion-reduce:transition-none ${isOpen(node) ? "rotate-90" : ""}`}
        aria-hidden="true"
      /></Button
    >
  {:else}<span class="size-6 shrink-0"></span>{/if}
{/snippet}

{#snippet row(node: TreeNode)}
  {@const status = node.session ? rowStatus(node.session) : undefined}
  {@const open = isOpen(node)}
  <li
    role="none"
    class={`flex items-center ${node.level === 3 ? "pl-8" : node.level === 2 ? "pl-4" : ""}`}
  >
    {#if node.kind !== "session"}{@render chevron(node)}{:else}<span
        class="size-6 shrink-0"
      ></span>{/if}
    <div class="group/row relative min-w-0 flex-1">
      <ListButton
        variant="file-tree"
        role="treeitem"
        aria-level={node.level}
        aria-expanded={node.children.length && !node.reason ? open : undefined}
        aria-selected={isCurrent(node)}
        aria-current={isCurrent(node) ? "page" : undefined}
        aria-label={status
          ? `${node.label}（${rowStatusLabels[status]}）`
          : node.reason
            ? `${node.label}（不可用）`
            : node.waiting
              ? `${node.label}，${node.count} 个会话，${node.waiting} 个等待`
              : node.count
                ? `${node.label}，${node.count} 个会话`
                : node.label}
        title={node.reason
          ? `${node.label}：${node.reason}`
          : status
            ? `${node.label}（${rowStatusLabels[status]}）`
            : (node.path ?? node.label)}
        tabindex={tabStop === node.key ? 0 : -1}
        data-tree-key={node.key}
        onfocus={() => (focusKey = node.key)}
        onkeydown={(event: KeyboardEvent) => keydown(event, node)}
        onclick={() => activate(node)}
      >
        {#if node.kind === "all"}<IconLayoutDashboard aria-hidden="true" />
        {:else if node.kind === "repository"}<IconBook2 aria-hidden="true" />
        {:else if node.kind === "direct"}<IconFolder aria-hidden="true" />
        {:else if node.kind === "worktree" || node.kind === "missing"}<IconGitBranch
            aria-hidden="true"
          />
        {:else if node.session}<LaunchConfigIcon
            icon={sessionIcon(node.session)}
            size="xs"
          />{/if}
        <span
          class={`min-w-0 flex-1 truncate ${node.kind === "repository" ? "font-medium" : ""} ${node.reason || status === "ended" ? "text-muted-foreground" : ""}`}
          >{node.label}</span
        >
        {#if node.reason}<IconAlertCircle
            class="text-warning"
            aria-hidden="true"
          />
        {:else if status}<RowStatus {status} />
        {:else}{@render counts(node)}{/if}
      </ListButton>
      {#if node.kind === "repository" && !node.reason && canCreate}
        <ListButton
          variant="file-tree-action"
          aria-label={`在 ${node.label} 新建工作树`}
          title={`在 ${node.label} 新建工作树`}
          tabindex={tabStop === node.key ? 0 : -1}
          onclick={() => onnewworktree(node.repositoryId ?? "")}
          ><IconPlus aria-hidden="true" /></ListButton
        >
      {:else if node.session}
        <SidebarSessionMenu
          label={node.label}
          tabindex={tabStop === node.key ? 0 : -1}
          target={{ kind: "live", session: node.session, onrename, onclose }}
        />
      {/if}
    </div>
  </li>
  {#if open}
    <li role="none">
      <ul role="group" class="space-y-0.5">
        {#each node.children as child (child.key)}{@render row(child)}{/each}
      </ul>
    </li>
  {/if}
{/snippet}

<ul
  bind:this={root}
  role="tree"
  aria-label="仓库与会话"
  class="space-y-0.5 text-sm"
>
  {#each nodes as node (node.key)}{@render row(node)}{/each}
</ul>
