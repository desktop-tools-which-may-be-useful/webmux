<script lang="ts">
  import InlineNotice from "../components/InlineNotice.svelte";
  import LazyDialog from "../components/LazyDialog.svelte";
  import { onMount } from "svelte";
  import SessionViewControls from "../components/SessionViewControls.svelte";
  import WorkspaceToolbar from "../components/WorkspaceToolbar.svelte";
  import { store, stored } from "../api";
  import type { Repository, Worktree } from "../schemas";
  import PendingUpdateNotice from "../components/PendingUpdateNotice.svelte";
  import { dismissUpdate, dismissedUpdate, homeUpdate } from "../pendingUpdate";
  import { Button } from "$lib/components/ui/button";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import UnregisterRepositoryDialog from "../components/UnregisterRepositoryDialog.svelte";
  import SessionFilters from "../components/SessionFilters.svelte";
  import RepositoryGroup from "../components/RepositoryGroup.svelte";
  import DirectSessions from "../components/DirectSessions.svelte";
  import SidebarButton from "../components/SidebarButton.svelte";
  import { removeRecord, resumeHybrid } from "../recoverable";
  import {
    repositoryRecords,
    matchesArchivedQuery,
    placeArchived,
    type ArchiveActions,
  } from "../archived";
  import SessionListEmpty from "../components/SessionListEmpty.svelte";
  import { homeWorktrees } from "../homeWorktrees.svelte";
  import { setPinned, worktreeTitle } from "../homeWorktrees";
  import { matchesSessionQuery } from "../session-location";
  import { matchesStatusFilter, type StatusFilter } from "../session-status";
  import { resetBrowserChrome, setBrowserChrome } from "../browser-chrome";
  import { getShell } from "../shell.svelte";
  import { z } from "zod";

  // The workbench home: every live session grouped by repository and
  // worktree, beside the shell's tree. The shell owns the shared data.
  const shell = getShell();
  const { drafts } = shell;
  const sessions = $derived(shell.sessions.sessions);
  const recoverable = $derived(shell.archived.records);
  const repositories = $derived(shell.repositories.list);
  const repositoriesSettled = $derived(shell.repositories.settled);
  const refreshing = $derived(shell.repositories.refreshing);
  const repositoryError = $derived(shell.repositories.error);
  const setup = $derived(shell.setup.value);
  const gitReady = $derived(setup.gitReady);
  const registryReady = $derived(setup.registryReady);
  const capabilityError = $derived(setup.capabilityError);
  const loading = $derived(shell.sessions.loading);
  const initializing = $derived(!shell.setup.settled);
  const error = $derived(shell.sessions.error);
  let dismissedVersion = $state(dismissedUpdate());
  const updateNotice = $derived(
    homeUpdate(setup.pendingUpdate, dismissedVersion),
  );
  let removing = $state<Worktree | null>(null);
  let unregistering = $state<Repository | null>(null);
  let unregisterOpen = $state(false);
  let now = $state(Date.now());
  let focusedExpanded = $state<Record<string, boolean>>({});
  let expansionOverrides = $state<Record<string, boolean>>({});
  let query = $state("");
  let statusFilter = $state<StatusFilter>("all");
  let filterExpanded = $state<Record<string, boolean>>({});
  const repositoryChoice = $derived(shell.repositories.choice);
  // Output previews are the default; only an explicit choice keeps rows compact.
  let compact = $state(
    z.enum(["compact", "preview"]).safeParse(stored("homeSessionView")).data ===
      "compact",
  );
  // Closed or interrupted hybrid shells show as archived records only.
  const list = $derived(
    sessions.filter(
      (session) => !session.hybrid || session.hybrid.state === "active",
    ),
  );
  const direct = $derived(
    list.filter(
      (s) =>
        !s.workspace ||
        !repositories.some((r) => r.id === s.workspace?.repositoryId),
    ),
  );
  // Archived records sit in the group a live session of theirs would.
  const placement = $derived(placeArchived(recoverable, repositories));
  const selectedRepository = $derived(
    repositoryChoice &&
      (repositoryChoice === "direct" ||
        repositories.some((repo) => repo.id === repositoryChoice))
      ? repositoryChoice
      : "all",
  );
  // A worktree picked in the tree narrows the repository view to it.
  const selectedWorktree = $derived(
    repositories
      .find((repo) => repo.id === selectedRepository)
      ?.worktrees.find((tree) => tree.id === shell.repositories.worktree),
  );
  const scopeLabel = $derived(
    selectedWorktree
      ? `${repositories.find((repo) => repo.id === selectedRepository)?.name} · ${worktreeTitle(selectedWorktree)}`
      : selectedRepository === "direct"
        ? "直接会话"
        : (repositories.find((repo) => repo.id === selectedRepository)?.name ??
          "工作区"),
  );
  const scopedSessions = $derived(
    selectedRepository === "all"
      ? list
      : selectedRepository === "direct"
        ? direct
        : list.filter(
            (session) =>
              session.workspace?.repositoryId === selectedRepository &&
              (!selectedWorktree ||
                session.workspace.worktreeId === selectedWorktree.id),
          ),
  );
  const scopedArchive = $derived.by(() => {
    if (selectedRepository === "all") return recoverable;
    if (selectedRepository === "direct") return placement.direct;
    const entry = placement.repositories.get(selectedRepository);
    if (selectedWorktree)
      return [...(entry?.worktrees.get(selectedWorktree.id) ?? [])];
    return entry ? repositoryRecords(entry) : [];
  });
  const scopedArchiveCount = $derived(scopedArchive.length);
  $effect(() => {
    if (loading || initializing || !repositoriesSettled || repositoryError)
      return;
    if (!repositoryChoice) {
      shell.repositories.select(
        repositories.length > 1
          ? ((
              repositories.find((repo) =>
                list.some(
                  (session) => session.workspace?.repositoryId === repo.id,
                ),
              ) ?? repositories[0]
            )?.id ?? "all")
          : "all",
      );
    } else if (
      repositoryChoice !== "all" &&
      repositoryChoice !== "direct" &&
      !repositories.some((repo) => repo.id === repositoryChoice)
    ) {
      shell.repositories.select("all");
    }
  });
  function setCompact(value: boolean) {
    compact = value;
    store("homeSessionView", value ? "compact" : "preview");
  }
  const search = $derived(query.trim().toLocaleLowerCase());
  const filtering = $derived(Boolean(search) || statusFilter !== "all");
  const visible = $derived(
    scopedSessions.filter((session) => {
      if (!matchesStatusFilter(session.status, statusFilter)) return false;
      if (!search) return true;
      const repository = repositories.find(
        (repo) => repo.id === session.workspace?.repositoryId,
      );
      const tree = repository?.worktrees.find(
        (worktree) => worktree.id === session.workspace?.worktreeId,
      );
      return matchesSessionQuery(
        session,
        search,
        repository?.name,
        tree?.branch,
      );
    }),
  );
  const visibleIds = $derived(
    filtering || selectedRepository !== "all"
      ? new Set(visible.map((session) => session.id))
      : undefined,
  );
  // Archived records the filters keep: the archived status or none, and the
  // search over their titles, agent, directory, repository and branch.
  const visibleArchiveIds = $derived.by(() => {
    if (!filtering) return undefined;
    if (statusFilter !== "all" && statusFilter !== "archived")
      return new Set<string>();
    return new Set(
      recoverable
        .filter((record) => {
          if (!search) return true;
          const repository = repositories.find(
            (repo) => repo.id === record.workspace?.repositoryId,
          );
          const tree = repository?.worktrees.find(
            (worktree) => worktree.id === record.workspace?.worktreeId,
          );
          return matchesArchivedQuery(
            record,
            search,
            repository?.name,
            tree?.branch,
          );
        })
        .map((record) => record.id),
    );
  });
  const archivedMatches = $derived(
    visibleArchiveIds === undefined
      ? 0
      : scopedArchive.filter((record) => visibleArchiveIds.has(record.id))
          .length,
  );
  function archivedInRepository(id: string) {
    const entry = placement.repositories.get(id);
    return Boolean(
      entry &&
      repositoryRecords(entry).some(
        (record) => !visibleArchiveIds || visibleArchiveIds.has(record.id),
      ),
    );
  }
  const archive: ArchiveActions = $derived({
    now,
    filtering,
    resume: resumeHybrid,
    remove: async (id) => {
      await removeRecord(id);
      drafts.remove(id);
      await shell.archived.refresh();
    },
    removeWorktree: (tree) => {
      removing = tree;
    },
    onresumed: () => void shell.archived.refresh(),
  });
  // Only an actively producing session follows the tab indicator; idle live
  // sessions are still "active" for the list summary but not for this marker.
  const running = $derived(
    sessions.some((session) => session.status === "running"),
  );
  $effect(() => {
    setBrowserChrome("WebMux · 工作区", running);
  });
  $effect(() => {
    // A new query opens matching repositories, while still allowing the user
    // to collapse them without changing the unfiltered workspace layout.
    void query;
    void statusFilter;
    void selectedRepository;
    void selectedWorktree;
    filterExpanded = {};
  });
  function clearFilters() {
    query = "";
    statusFilter = "all";
  }
  const expanded = $derived.by(() =>
    Object.fromEntries(
      repositories.map((repo) => [
        repo.id,
        expansionOverrides[repo.id] ??
          (sessions.some((s) => s.workspace?.repositoryId === repo.id) ||
            placement.repositories.has(repo.id)),
      ]),
    ),
  );
  function pin(repositoryId: string, ids: string[], pinned: boolean) {
    const repo = repositories.find((r) => r.id === repositoryId);
    if (repo)
      homeWorktrees.save(setPinned(homeWorktrees.value, repo, ids, pinned));
  }
  // After a worktree change: a removal also forgets the archived records of
  // sessions in the removed directory.
  function refreshWorktrees() {
    void shell.repositories.refresh();
    void shell.archived.refresh().then((records) => {
      if (records)
        drafts.reconcile([
          ...sessions.map((session) => session.id),
          ...records,
        ]);
    });
  }
  onMount(() => {
    // Coming home re-reads Git, as the page did before the shell kept the
    // last snapshot; the snapshot stays on screen meanwhile.
    void shell.repositories.refresh();
    const visible = () => {
      if (!document.hidden) {
        now = Date.now();
        void shell.repositories.refresh();
      }
    };
    document.addEventListener("visibilitychange", visible);
    window.addEventListener("pageshow", visible);
    const clock = setInterval(() => {
      now = Date.now();
    }, 30000);
    return () => {
      resetBrowserChrome();
      clearInterval(clock);
      document.removeEventListener("visibilitychange", visible);
      window.removeEventListener("pageshow", visible);
    };
  });
</script>

<div class="workspace-page safe-area-page min-h-dvh">
  <!-- Phones reach the sidebar from this bar; wider screens show it. -->
  <header
    class="app-header flex min-h-(--app-header-height) items-center gap-1 border-b bg-card px-1 sm:hidden"
  >
    <SidebarButton />
    <h1 class="min-w-0 truncate text-sm font-medium">
      {selectedRepository === "all" ? "全部仓库" : scopeLabel}
    </h1>
    <!-- The sidebar footer is out of sight here, so phones keep this notice. -->
    {#if shell.sessions.connection === "reconnecting"}<span
        role="status"
        class="ml-auto shrink-0 pr-2 text-xs text-warning"
        >连接已断开，重连中…</span
      >{/if}
  </header>
  {#if updateNotice}
    {@const version = updateNotice.installedVersion}
    <div class="mx-auto max-w-7xl px-4 pt-5 sm:px-8 sm:pt-6">
      <PendingUpdateNotice
        update={updateNotice}
        ondismiss={() => {
          dismissUpdate(version);
          dismissedVersion = version;
        }}
      />
    </div>
  {/if}
  {#if loading || initializing}
    <div
      class="mx-auto max-w-7xl space-y-4 px-4 py-5 sm:px-8 sm:py-6"
      aria-label="正在加载会话"
    >
      {#each [0, 1, 2] as item (item)}<Skeleton class="h-40 w-full" />{/each}
    </div>
  {:else}
    <div class="mx-auto max-w-6xl px-4 py-5 sm:px-8 sm:py-6">
      <main class="min-w-0">
        <WorkspaceToolbar
          title={scopeLabel}
          {loading}
          {filtering}
          visibleCount={visible.length}
          sessionCount={scopedSessions.length}
          repositoryCount={selectedRepository === "all"
            ? repositories.length
            : selectedRepository === "direct"
              ? 0
              : 1}
          {refreshing}
          onsettings={shell.settingsRepository
            ? shell.openRepositorySettings
            : undefined}
          onrefresh={() => void shell.repositories.refresh(true)}
          onstart={() => shell.start()}
        >
          {#snippet controls()}<SessionViewControls
              {compact}
              onchange={setCompact}
            />{/snippet}
          <SessionFilters
            sessions={scopedSessions}
            archivedCount={scopedArchiveCount}
            bind:query
            bind:statusFilter
            filtered={filtering && (visible.length > 0 || archivedMatches > 0)}
            shortcutDisabled={Boolean(removing || unregisterOpen)}
            onclear={clearFilters}
          />
        </WorkspaceToolbar>
        {#if shell.setup.error}<InlineNotice tone="warning" class="mb-4">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span>{shell.setup.error}</span>
              <Button
                variant="ghost"
                size="sm"
                class="min-h-9 sm:min-h-8"
                disabled={shell.setup.busy}
                onclick={() => void shell.setup.load()}
              >
                {shell.setup.busy ? "正在重试…" : "重试"}
              </Button>
            </div>
          </InlineNotice>{/if}
        {#if error}<InlineNotice tone="danger" class="mb-4">
            {error}
          </InlineNotice>{/if}
        {#if capabilityError}<InlineNotice tone="warning" class="mb-4">
            仓库管理部分不可用：{capabilityError}。仍可直接启动会话。
          </InlineNotice>{/if}
        {#if repositoryError}<InlineNotice tone="danger" class="mb-4">
            {repositories.length
              ? `Git 状态刷新失败：${repositoryError}。保留上次快照与全部会话。`
              : `Git 状态读取失败：${repositoryError}。会话仍可正常使用。`}
          </InlineNotice>{/if}
        <div class="space-y-8">
          {#each repositories as repo (repo.id)}
            <RepositoryGroup
              {repo}
              sessions={list.filter(
                (s) => s.workspace?.repositoryId === repo.id,
              )}
              {filtering}
              {compact}
              focused={selectedRepository === repo.id}
              worktree={selectedRepository === repo.id
                ? selectedWorktree?.id
                : undefined}
              hidden={selectedRepository !== "all" &&
                selectedRepository !== repo.id}
              {visibleIds}
              open={filtering
                ? (visible.some(
                    (session) => session.workspace?.repositoryId === repo.id,
                  ) ||
                    archivedInRepository(repo.id)) &&
                  (filterExpanded[repo.id] ?? true)
                : selectedRepository === repo.id
                  ? (focusedExpanded[repo.id] ?? true)
                  : (expanded[repo.id] ?? false)}
              onopenchange={(open) => {
                if (filtering) filterExpanded[repo.id] = open;
                else if (selectedRepository === repo.id)
                  focusedExpanded[repo.id] = open;
                else expansionOverrides[repo.id] = open;
              }}
              {gitReady}
              {registryReady}
              {refreshing}
              {now}
              onstart={(worktreeId) =>
                worktreeId === undefined
                  ? shell.start({
                      kind: "new-worktree",
                      repositoryId: repo.id,
                    })
                  : shell.start({
                      kind: "worktree",
                      repositoryId: repo.id,
                      worktreeId,
                    })}
              onunregister={() => {
                unregistering = repo;
                unregisterOpen = true;
              }}
              ontogglepin={(worktreeId, pinned) =>
                pin(repo.id, [worktreeId], pinned)}
              onremove={(tree) => {
                removing = tree;
              }}
              onclose={shell.close}
              archived={placement.repositories.get(repo.id)}
              {visibleArchiveIds}
              {archive}
            />
          {/each}
          {#if direct.length || placement.direct.length}<DirectSessions
              sessions={direct}
              {compact}
              focused={selectedRepository === "direct"}
              {visibleIds}
              {now}
              onclose={shell.close}
              archived={placement.direct}
              visibleArchiveIds={selectedRepository === "all" ||
              selectedRepository === "direct"
                ? visibleArchiveIds
                : new Set()}
              {archive}
            />{/if}
          {#if (filtering && !visible.length && !archivedMatches) || (!repositories.length && !list.length && !recoverable.length) || (selectedRepository === "direct" && !direct.length && !placement.direct.length)}<SessionListEmpty
              filtered={filtering && !visible.length && !archivedMatches}
              canStart
              onclear={clearFilters}
              onstart={() => shell.start()}
            />{/if}
        </div>
      </main>
    </div>
  {/if}
</div>
{#if removing}
  {@const target = removing}
  <LazyDialog
    load={() => import("../components/RemoveWorktreeDialog.svelte")}
    label="删除确认"
    onclose={() => (removing = null)}
  >
    {#snippet children(page)}<page.default
        worktree={target}
        onchange={refreshWorktrees}
        onclose={() => (removing = null)}
      />{/snippet}
  </LazyDialog>
{/if}
<UnregisterRepositoryDialog
  repository={unregistering}
  bind:open={unregisterOpen}
  onchange={() => shell.repositories.refresh(true)}
/>
