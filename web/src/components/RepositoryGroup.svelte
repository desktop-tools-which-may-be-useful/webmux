<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconCopy from "~icons/tabler/copy";
  import IconInfoCircle from "~icons/tabler/info-circle";
  import IconListCheck from "~icons/tabler/list-check";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import * as Popover from "$lib/components/ui/popover";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import type {
    ArchivedSession,
    Repository,
    SessionInfo,
    Worktree,
  } from "../schemas";
  import {
    EMPTY_ARCHIVE,
    repositoryRecords,
    type ArchiveActions,
    type RepositoryArchive,
  } from "../archived";
  import { homeWorktrees } from "../home-worktrees.svelte";
  import { homeLayout, worktreeTitle } from "../home-worktrees";
  import { statusSummary } from "../session-status";
  import { copyText } from "../clipboard";
  import HomeWorktreePicker from "./HomeWorktreePicker.svelte";
  import WorkspaceGroup from "./WorkspaceGroup.svelte";

  let {
    repo,
    sessions,
    visibleIds,
    filtering,
    hidden = false,
    focused = false,
    worktree,
    compact = false,
    open,
    onopenchange,
    gitReady,
    registryReady,
    refreshing,
    now,
    onstart,
    onmanage,
    onunregister,
    ontogglepin,
    onremove,
    onclose,
    archived = EMPTY_ARCHIVE,
    visibleArchiveIds,
    archive,
  }: {
    repo: Repository;
    // All sessions associated with this repository.
    sessions: SessionInfo[];
    visibleIds?: ReadonlySet<string>;
    filtering: boolean;
    hidden?: boolean;
    focused?: boolean;
    /** Shows only this worktree's group (picked in the workbench tree). */
    worktree?: string;
    compact?: boolean;
    open: boolean;
    onopenchange: (open: boolean) => void;
    gitReady: boolean;
    registryReady: boolean;
    refreshing: boolean;
    now: number;
    // Without a worktree id the sheet creates a new worktree.
    onstart: (worktreeId?: string) => void;
    onmanage?: () => void;
    onunregister: () => void;
    ontogglepin: (worktreeId: string, pinned: boolean) => void;
    onremove: (worktree: Worktree) => void;
    onclose: (session: SessionInfo) => void;
    // Every archived record placed in this repository.
    archived?: RepositoryArchive;
    // Archived records the filters keep; undefined while not filtering.
    visibleArchiveIds?: ReadonlySet<string>;
    archive?: ArchiveActions;
  } = $props();

  let detailsOpen = $state(false);
  $effect(() => {
    if (hidden || (filtering && !visible.length && !archivedVisible))
      detailsOpen = false;
  });
  let contentReady = $state(false);
  $effect(() => {
    if (open) contentReady = true;
  });

  const visible = $derived(
    visibleIds
      ? sessions.filter((session) => visibleIds.has(session.id))
      : sessions,
  );
  const missing = $derived(
    sessions.filter(
      (s) => !repo.worktrees.some((w) => w.id === s.workspace?.worktreeId),
    ),
  );
  // Archived records of a group that the filters keep.
  function shownArchive(records: readonly ArchivedSession[] = []) {
    return visibleArchiveIds
      ? records.filter((record) => visibleArchiveIds.has(record.id))
      : [...records];
  }
  const archivedVisible = $derived(
    shownArchive(repositoryRecords(archived)).length > 0,
  );
  const renderLayout = $derived(
    homeLayout(homeWorktrees.value, repo, sessions, archived.worktrees),
  );
  const layout = $derived(
    visibleIds
      ? homeLayout(homeWorktrees.value, repo, visible, archived.worktrees)
      : renderLayout,
  );
  // Every worktree in a stable order, so a row does not jump when its check
  // changes; a worktree with sessions is listed even when not pinned.
  const sortedTrees = $derived(renderLayout.trees);
  const pinnedIds = $derived(
    new Set(renderLayout.shown.filter((w) => w.pinned).map((w) => w.tree.id)),
  );
  const activeIds = $derived(
    new Set(renderLayout.shown.filter((w) => !w.pinned).map((w) => w.tree.id)),
  );
</script>

<Collapsible.Root
  class="min-w-0 space-y-4"
  data-repository-id={repo.id}
  hidden={hidden || (filtering && !visible.length && !archivedVisible)}
  role="region"
  aria-label={`仓库 ${repo.name}`}
  {open}
  onOpenChange={onopenchange}
>
  <div
    class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-3"
  >
    <h2 class="min-w-0 sm:flex-1">
      {#if focused}
        <span class="flex min-w-0 items-baseline gap-2 font-semibold">
          工作树
          <span class="text-xs font-normal text-muted-foreground tabular-nums"
            >主页 {layout.shown.length}/{repo.worktrees.length}</span
          >
        </span>
      {:else}
        <Collapsible.Trigger
          class="group flex min-w-0 max-w-full items-center gap-2 text-left font-semibold outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <IconChevronDown
            class="size-4 shrink-0 -rotate-90 transition-transform group-data-[state=open]:rotate-0"
            aria-hidden="true"
          />
          <span class="min-w-0 break-all">{repo.name}</span>
        </Collapsible.Trigger>
      {/if}
    </h2>
    <div
      class="ml-auto flex max-w-full flex-wrap items-center justify-end gap-1"
    >
      <HomeWorktreePicker
        repositoryName={repo.name}
        worktrees={sortedTrees}
        pinned={pinnedIds}
        active={activeIds}
        shown={layout.shown.length}
        ontoggle={ontogglepin}
        onmanage={repo.availability === "available" && gitReady
          ? onmanage
          : undefined}
      />
      {#if onmanage}
        <Button
          size="sm"
          variant="outline"
          class="min-h-9 sm:min-h-8"
          aria-label="管理工作树"
          title="管理工作树"
          disabled={repo.availability !== "available" || !gitReady}
          onclick={onmanage}
          ><IconListCheck class="size-4" aria-hidden="true" /><span
            class="hidden sm:inline">管理工作树</span
          ></Button
        >
      {/if}
      <Button
        size="sm"
        variant="outline"
        class="min-h-9 sm:min-h-8"
        disabled={repo.availability !== "available" || !gitReady}
        onclick={() => onstart()}
        ><IconPlus class="size-4" aria-hidden="true" />新建工作树</Button
      >
      <Popover.Root bind:open={detailsOpen}>
        <Popover.Trigger>
          {#snippet child({ props })}<Button
              {...props}
              size="icon-sm"
              variant="ghost"
              aria-label={`仓库详情 ${repo.name}`}
              title={`仓库详情 ${repo.name}`}
              ><IconInfoCircle aria-hidden="true" /></Button
            >{/snippet}
        </Popover.Trigger>
        <Popover.Content
          align="end"
          data-slot="repository-details"
          class="w-[min(24rem,calc(100vw-2rem))] space-y-3"
        >
          <Popover.Header
            ><Popover.Title>{repo.name}</Popover.Title></Popover.Header
          >
          <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-xs">
            <dt class="text-muted-foreground">路径</dt>
            <dd class="flex min-w-0 items-start gap-1">
              <span class="min-w-0 flex-1 font-mono break-all"
                >{repo.anchorPath}</span
              >
              <Button
                size="icon-xs"
                variant="ghost"
                aria-label="复制仓库路径"
                title="复制仓库路径"
                onclick={() => void copyText(repo.anchorPath)}
                ><IconCopy aria-hidden="true" /></Button
              >
            </dd>
            <dt class="text-muted-foreground">工作区</dt>
            <dd>主页 {layout.shown.length} / 共 {repo.worktrees.length}</dd>
            <dt class="text-muted-foreground">会话</dt>
            <dd>{statusSummary(visible)}</dd>
            <dt class="text-muted-foreground">Git</dt>
            <dd>
              {refreshing
                ? "正在重新读取状态…"
                : `读取于 ${new Date(repo.checkedAt).toLocaleString()}`}{repo.availability ===
              "unavailable"
                ? " · 不可用，状态未知"
                : ""}
            </dd>
          </dl>
          <div class="border-t pt-3">
            <Button
              size="sm"
              variant="destructive"
              disabled={!registryReady}
              onclick={() => {
                detailsOpen = false;
                onunregister();
              }}>从列表移除仓库…</Button
            >
          </div>
        </Popover.Content>
      </Popover.Root>
    </div>
    {#if repo.error}<p
        role="alert"
        class="w-full text-sm break-all text-destructive"
      >
        {repo.error}
      </p>{/if}
  </div>
  {#if contentReady}<Collapsible.Content
      forceMount
      hidden={!open}
      class="space-y-8"
    >
      {#each renderLayout.shown as { tree, pinned } (tree.id)}
        {@const treeArchive = shownArchive(archived.worktrees.get(tree.id))}
        <WorkspaceGroup
          title={worktreeTitle(tree)}
          path={tree.path}
          worktree={tree}
          sessions={sessions.filter((s) => s.workspace?.worktreeId === tree.id)}
          {visibleIds}
          {compact}
          hidden={hidden ||
            !open ||
            (!!worktree && worktree !== tree.id) ||
            (filtering &&
              !treeArchive.length &&
              !visible.some(
                (session) => session.workspace?.worktreeId === tree.id,
              ))}
          {now}
          {pinned}
          ontogglepin={() => ontogglepin(tree.id, !pinned)}
          {onclose}
          onstart={() => onstart(tree.id)}
          onremove={tree.kind === "linked" ? () => onremove(tree) : undefined}
          archived={treeArchive}
          {archive}
        />
      {/each}
      {@const missingArchive = shownArchive(archived.missing)}
      {#if missing.length || archived.missing.length}<WorkspaceGroup
          title="已不可用工作区"
          sessions={missing}
          {visibleIds}
          {compact}
          hidden={hidden ||
            !open ||
            !!worktree ||
            Boolean(
              visibleIds &&
              !missingArchive.length &&
              !missing.some((session) => visibleIds.has(session.id)),
            )}
          {now}
          {onclose}
          archived={missingArchive}
          {archive}
        />{/if}
      {#if !layout.shown.length && layout.unpinned.length}<p
          class="text-sm text-muted-foreground"
        >
          主页还没有 worktree。用上方“主页显示”选择，或“新建工作树”。
        </p>{/if}
      {#if !repo.worktrees.length && !missing.length && !archived.missing.length}<p
          class="text-sm text-muted-foreground"
        >
          {repo.error
            ? "无法读取工作树，请检查路径与 Git 权限，或从列表移除。"
            : "暂无可用工作树。"}
        </p>{/if}
    </Collapsible.Content>{/if}
</Collapsible.Root>
