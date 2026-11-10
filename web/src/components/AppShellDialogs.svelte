<script lang="ts">
  import { homeWorktrees } from "../home-worktrees.svelte";
  import { homeLayout, setPinned } from "../home-worktrees";
  import type { LaunchEntry, LaunchScope } from "../new-session-defaults";
  import type {
    ArchivedSession,
    Repository,
    SessionInfo,
    Worktree,
  } from "../schemas";
  import type { Shell } from "../shell.svelte";
  import CloseSessionDialog from "./CloseSessionDialog.svelte";
  import LazyDialog from "./LazyDialog.svelte";
  import RenameSessionDialog from "./RenameSessionDialog.svelte";
  import UnregisterRepositoryDialog from "./UnregisterRepositoryDialog.svelte";

  let {
    shell,
    creating = $bindable(null),
    renaming = $bindable(null),
    closing = $bindable(null),
    removingWorktree = $bindable(null),
    managingWorktrees = $bindable(null),
    choosingHome = $bindable(null),
    viewingRepository = $bindable(null),
    releasingWorktree = $bindable(null),
    forwarding = $bindable(false),
    adding = $bindable(false),
    configuring = $bindable(null),
    onrefreshworktrees,
  }: {
    shell: Shell;
    creating: { entry?: LaunchEntry; scope?: LaunchScope } | null;
    renaming: SessionInfo | null;
    closing: SessionInfo | null;
    removingWorktree: {
      worktree: Worktree;
      onremoved?: () => void;
      forgetRecords?: readonly ArchivedSession[];
    } | null;
    managingWorktrees: Repository | null;
    choosingHome: Repository | null;
    viewingRepository: Repository | null;
    releasingWorktree: Worktree | null;
    forwarding: boolean;
    adding: boolean;
    configuring: Repository | null;
    onrefreshworktrees: () => void;
  } = $props();

  // Kept after closing so the confirmation still names the repository.
  let unregistering = $state<Repository | null>(null);
  let unregisterOpen = $state(false);

  // Unpins a worktree from the home page; absent when its repository is gone.
  function hideFromHome(tree: Worktree) {
    const repo = shell.repositories.list.find(
      (r) => r.id === tree.repositoryId,
    );
    // Already off the home page: nothing to hide.
    const pinned = repo
      ? homeLayout(homeWorktrees.value, repo, [], new Map()).shown.some(
          (w) => w.tree.id === tree.id && w.pinned,
        )
      : false;
    return repo && pinned
      ? () =>
          homeWorktrees.save(
            setPinned(homeWorktrees.value, repo, [tree.id], false),
          )
      : undefined;
  }
</script>

{#if creating}
  {@const target = creating}
  <LazyDialog
    load={() => import("./NewSessionDialog.svelte")}
    label="创建表单"
    onclose={() => (creating = null)}
  >
    {#snippet children(page)}<page.default
        launchConfigs={shell.launchConfigs.state}
        repositories={[...shell.repositories.list]}
        defaultCwd={shell.setup.value.cwd}
        entry={target.entry}
        scope={target.scope}
        selectedRepositoryId={shell.repositories.choice ?? "all"}
        intent={target.entry?.kind === "new-worktree" ? "worktree" : "session"}
        onchange={onrefreshworktrees}
        onworktreecreated={(tree) => {
          const repo = shell.repositories.list.find(
            (r) => r.id === tree.repositoryId,
          );
          if (repo)
            homeWorktrees.save(
              setPinned(homeWorktrees.value, repo, [tree.id], true),
            );
        }}
        onclose={() => (creating = null)}
      />{/snippet}
  </LazyDialog>
{/if}
{#if configuring}
  {@const target = configuring}
  <LazyDialog
    load={() => import("./RepositorySettingsDialog.svelte")}
    label="仓库设置"
    onclose={() => (configuring = null)}
  >
    {#snippet children(page)}<page.default
        repository={target}
        onchange={() => void shell.repositories.refresh(true)}
        onclose={() => (configuring = null)}
      />{/snippet}
  </LazyDialog>
{/if}
{#if forwarding}
  <LazyDialog
    load={() => import("./ForwardDialog.svelte")}
    label="端口转发"
    onclose={() => (forwarding = false)}
  >
    {#snippet children(page)}<page.default
        sessions={[...shell.sessions.sessions]}
        detection={shell.setup.value.portDetection}
        onclose={() => (forwarding = false)}
      />{/snippet}
  </LazyDialog>
{/if}
{#if adding}
  <LazyDialog
    load={() => import("./AddRepositoryDialog.svelte")}
    label="仓库表单"
    onclose={() => (adding = false)}
  >
    {#snippet children(page)}<page.default
        defaultCwd={shell.setup.value.cwd}
        onchange={() => void shell.repositories.refresh(true)}
        onclose={() => (adding = false)}
      />{/snippet}
  </LazyDialog>
{/if}
{#if renaming}<RenameSessionDialog
    session={renaming}
    onclose={() => (renaming = null)}
  />{/if}
{#if closing}<CloseSessionDialog
    session={closing}
    onremoved={() => {
      if (!closing) return;
      shell.drafts.remove(closing.id);
      shell.archiveSignal.emit(closing.id);
    }}
    onarchived={(session) => {
      void shell.archived.refresh();
      shell.archiveSignal.emit(session.id);
    }}
    onclose={() => (closing = null)}
  />{/if}
{#if removingWorktree}
  {@const target = removingWorktree}
  <LazyDialog
    load={() => import("./RemoveWorktreeDialog.svelte")}
    label="删除确认"
    onclose={() => (removingWorktree = null)}
  >
    {#snippet children(page)}<page.default
        worktree={target.worktree}
        forgetRecords={target.forgetRecords ?? []}
        onremoved={target.onremoved}
        onhide={hideFromHome(target.worktree)}
        onchange={onrefreshworktrees}
        onclose={() => (removingWorktree = null)}
      />{/snippet}
  </LazyDialog>
{/if}
{#if managingWorktrees}
  {@const target = managingWorktrees}
  {@const currentRepo =
    shell.repositories.list.find((r) => r.id === target.id) ?? target}
  <LazyDialog
    load={() => import("./ManageWorktreesDialog.svelte")}
    label="管理工作树"
    onclose={() => (managingWorktrees = null)}
  >
    {#snippet children(page)}<page.default
        repository={currentRepo}
        sessions={shell.sessions.sessions}
        archived={shell.archived.records}
        home={homeWorktrees.value}
        onremoved={(removedIds) => {
          homeWorktrees.save(
            setPinned(homeWorktrees.value, currentRepo, removedIds, false),
          );
        }}
        onchange={onrefreshworktrees}
        onclose={() => (managingWorktrees = null)}
      />{/snippet}
  </LazyDialog>
{/if}
{#if choosingHome}
  {@const target = choosingHome}
  {@const currentRepo =
    shell.repositories.list.find((r) => r.id === target.id) ?? target}
  <LazyDialog
    load={() => import("./HomeWorktreeDialog.svelte")}
    label="主页显示"
    onclose={() => (choosingHome = null)}
  >
    {#snippet children(page)}<page.default
        repository={currentRepo}
        sessions={shell.sessions.sessions}
        records={shell.archived.records}
        home={homeWorktrees.value}
        ontoggle={(worktreeId, pinned) =>
          homeWorktrees.save(
            setPinned(homeWorktrees.value, currentRepo, [worktreeId], pinned),
          )}
        onmanage={() => {
          choosingHome = null;
          shell.manageWorktrees(currentRepo);
        }}
        onclose={() => (choosingHome = null)}
      />{/snippet}
  </LazyDialog>
{/if}
{#if viewingRepository}
  {@const target = viewingRepository}
  {@const currentRepo =
    shell.repositories.list.find((r) => r.id === target.id) ?? target}
  <LazyDialog
    load={() => import("./RepositoryDetailsDialog.svelte")}
    label="仓库详情"
    onclose={() => (viewingRepository = null)}
  >
    {#snippet children(page)}<page.default
        repository={currentRepo}
        sessions={shell.sessions.sessions}
        records={shell.archived.records}
        home={homeWorktrees.value}
        refreshing={shell.repositories.refreshing}
        registryReady={shell.setup.value.registryReady}
        onunregister={() => {
          unregistering = currentRepo;
          unregisterOpen = true;
        }}
        onclose={() => (viewingRepository = null)}
      />{/snippet}
  </LazyDialog>
{/if}
<UnregisterRepositoryDialog
  repository={unregistering}
  bind:open={unregisterOpen}
  onchange={() => shell.repositories.refresh(true)}
/>
{#if releasingWorktree}
  {@const target = releasingWorktree}
  <LazyDialog
    load={() => import("./ReleaseWorktreeDialog.svelte")}
    label="释放空间预览"
    onclose={() => (releasingWorktree = null)}
  >
    {#snippet children(page)}<page.default
        worktree={target}
        onchange={onrefreshworktrees}
        onclose={() => (releasingWorktree = null)}
      />{/snippet}
  </LazyDialog>
{/if}
