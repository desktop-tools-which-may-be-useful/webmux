<script lang="ts" module>
  import type { Branches, Remotes } from "../schemas";

  /** Where the panel reads branches and remotes; stories pass fixtures. */
  export interface RepositorySource {
    branches(id: string, signal: AbortSignal): Promise<Branches>;
    remotes(id: string, signal: AbortSignal): Promise<Remotes>;
  }
</script>

<script lang="ts">
  import { onDestroy, tick, untrack } from "svelte";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import { Button } from "$lib/components/ui/button";
  import * as Tabs from "$lib/components/ui/tabs";
  import { repositoryBranches, repositoryRemotes } from "../api";
  import { errorMessage } from "../error";
  import { LatestRequest } from "../hooks/remote-request.svelte";
  import { worktreeTitle } from "../home-worktrees";
  import type {
    ArchivedSession,
    HomeWorktrees,
    Remote,
    Repository,
    SessionInfo,
  } from "../schemas";
  import type { RepositoryTab } from "../shell.svelte";
  import InlineNotice from "./InlineNotice.svelte";
  import RepositoryBranches from "./RepositoryBranches.svelte";
  import RepositoryOverview from "./RepositoryOverview.svelte";
  import RepositoryRemotes from "./RepositoryRemotes.svelte";
  import RepositoryWorktrees from "./RepositoryWorktrees.svelte";
  import ResponsiveDialog from "./ResponsiveDialog.svelte";
  import WorktreeDetails from "./WorktreeDetails.svelte";

  // The repository panel: overview, worktrees, branches and remotes as line
  // tabs in one dialog (a bottom drawer on phones). A worktree's details open
  // inside it as a subview with "返回" instead of a second dialog; the tabs
  // stay mounted underneath, so their search, filters and scroll survive.
  // While a section's request is in flight (`busy`), going back, switching
  // sections and closing are all blocked.
  let {
    repository,
    sessions,
    records,
    home,
    refreshing,
    refreshError,
    registryReady,
    initialTab = "overview",
    source = { branches: repositoryBranches, remotes: repositoryRemotes },
    onunregister,
    onchange,
    onremoved,
    onclose,
  }: {
    repository: Repository;
    sessions: readonly SessionInfo[];
    records: readonly ArchivedSession[];
    home: HomeWorktrees;
    /** The repositories are being read again; this is the last snapshot. */
    refreshing: boolean;
    /** The last read's failure; "" after a successful one. */
    refreshError: string;
    registryReady: boolean;
    initialTab?: RepositoryTab;
    source?: RepositorySource;
    onunregister: () => void;
    onchange: () => void;
    onremoved?: (removedIds: string[]) => void;
    onclose: () => void;
  } = $props();

  const available = $derived(repository.availability === "available");
  let tab = $state<RepositoryTab>(untrack(() => initialTab));
  let busy = $state(false);
  // The worktree whose details are shown over the tabs; null for the tabs.
  let detail = $state<string | null>(null);
  const detailTree = $derived(
    detail
      ? repository.worktrees.find((tree) => tree.id === detail)
      : undefined,
  );
  const tabs: { id: RepositoryTab; label: string }[] = [
    { id: "overview", label: "概览" },
    { id: "worktrees", label: "工作树" },
    { id: "branches", label: "分支" },
    { id: "remotes", label: "远程" },
  ];
  const descriptions: Record<RepositoryTab, string> = {
    overview: "仓库路径、默认分支、远程与 Git 状态。",
    worktrees:
      "该仓库的全部工作树（含未添加到主页的项），可查看详情或批量清理。删除工作树不会删除本地分支。",
    branches: "本地分支与远程跟踪分支，按本机记录读取，不访问网络。",
    remotes: "远程名称与地址；地址中的密码与令牌已隐藏。",
  };

  // Remotes are read once when the overview (their count) or their section
  // is first shown.
  let remotes = $state<Remote[] | undefined>();
  let remotesError = $state("");
  const remoteRequests = new LatestRequest();
  onDestroy(() => remoteRequests.dispose());
  async function readRemotes() {
    const request = remoteRequests.begin();
    remotesError = "";
    try {
      const result = await source.remotes(repository.id, request.signal);
      if (request.current()) remotes = result.remotes;
    } catch (err) {
      if (request.reportable()) remotesError = errorMessage(err);
    }
  }
  let remotesAsked = false;
  $effect(() => {
    if (!available || remotesAsked) return;
    if (tab !== "overview" && tab !== "remotes") return;
    remotesAsked = true;
    untrack(() => void readRemotes());
  });

  // Scroll offsets of the tabs and the dialog body, restored on "返回".
  let body = $state<HTMLElement>();
  let tabsView = $state<HTMLElement>();
  let saved: [HTMLElement, number][] = [];
  function dialogScrollers(): HTMLElement[] {
    const out: HTMLElement[] = [];
    for (let el = body?.parentElement; el; el = el.parentElement) {
      out.push(el);
      if (el.getAttribute("role") === "dialog") break;
    }
    return out;
  }
  async function openWorktree(id: string) {
    if (busy) return;
    const outer = dialogScrollers();
    saved = [...(tabsView?.querySelectorAll<HTMLElement>("*") ?? []), ...outer]
      .filter((el) => el.scrollTop > 0)
      .map((el) => [el, el.scrollTop]);
    detail = id;
    await tick();
    for (const el of outer) el.scrollTop = 0;
  }
  function openPath(path: string) {
    const tree = repository.worktrees.find((w) => w.path === path);
    if (tree) void openWorktree(tree.id);
  }
  async function back() {
    if (busy) return;
    detail = null;
    await tick();
    for (const [el, top] of saved) el.scrollTop = top;
    saved = [];
  }
  const worktreePaths = $derived(
    new Set(repository.worktrees.map((tree) => tree.path)),
  );
</script>

<ResponsiveDialog
  title={repository.name}
  description={available ? descriptions[tab] : descriptions.overview}
  class="sm:max-w-2xl"
  drawerDescription="概览、工作树、分支与远程"
  pinDrawerHeader
  truncateDrawerTitle
  {busy}
  {onclose}
>
  <div bind:this={body} class="min-w-0 space-y-3">
    {#if refreshing}
      <p role="status" class="text-xs text-muted-foreground">
        正在重新读取状态…（先显示上次读取的结果）
      </p>
    {:else if refreshError}
      <InlineNotice tone="danger"
        >重新读取失败：{refreshError}。以下为 {new Date(
          repository.checkedAt,
        ).toLocaleString()} 读取的状态，可能已过时。</InlineNotice
      >
    {/if}
    <div bind:this={tabsView} hidden={!!detail}>
      {#if available}
        <Tabs.Root
          value={tab}
          onValueChange={(value) => {
            const next = tabs.find((item) => item.id === value);
            if (next && !busy) tab = next.id;
          }}
        >
          <!-- Wraps instead of overflowing with large text on a narrow phone. -->
          <Tabs.List
            variant="line"
            class="max-w-full flex-wrap group-data-horizontal/tabs:h-auto"
            aria-label="仓库面板分区"
          >
            {#each tabs as item (item.id)}
              <Tabs.Trigger value={item.id} disabled={busy && tab !== item.id}
                >{item.label}</Tabs.Trigger
              >
            {/each}
          </Tabs.List>
          <Tabs.Content value="overview" class="pt-2">
            <RepositoryOverview
              {repository}
              {sessions}
              {records}
              {home}
              remoteCount={remotes?.length}
              {registryReady}
              {onunregister}
            />
          </Tabs.Content>
          <Tabs.Content value="worktrees" class="pt-2">
            <RepositoryWorktrees
              {repository}
              {sessions}
              archived={records}
              {home}
              bind:busy
              onopen={(id) => void openWorktree(id)}
              {onchange}
              {onremoved}
            />
          </Tabs.Content>
          <Tabs.Content value="branches" class="pt-2">
            <RepositoryBranches
              active={tab === "branches"}
              load={(signal) => source.branches(repository.id, signal)}
              {worktreePaths}
              onopen={openPath}
            />
          </Tabs.Content>
          <Tabs.Content value="remotes" class="pt-2">
            <RepositoryRemotes
              {remotes}
              error={remotesError}
              onretry={() => void readRemotes()}
            />
          </Tabs.Content>
        </Tabs.Root>
      {:else}
        <RepositoryOverview
          {repository}
          {sessions}
          {records}
          {home}
          remoteCount={undefined}
          {registryReady}
          {onunregister}
        />
      {/if}
    </div>
    {#if detail}
      <section
        class="min-w-0 space-y-3"
        aria-label={detailTree
          ? `工作树详情：${worktreeTitle(detailTree)}`
          : "工作树详情"}
      >
        <div class="flex min-w-0 items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            class="-ml-2"
            disabled={busy}
            onclick={() => void back()}
            ><IconArrowLeft aria-hidden="true" />返回</Button
          >
          <h3 class="min-w-0 text-sm font-medium break-all">
            {detailTree ? worktreeTitle(detailTree) : "工作树"}
          </h3>
        </div>
        {#if detailTree}
          <WorktreeDetails
            path={detailTree.path}
            worktree={detailTree}
            sessions={sessions.filter(
              (s) => s.workspace?.worktreeId === detailTree.id,
            )}
            lastFetchedAt={repository.lastFetchedAt}
          />
        {:else}
          <p class="text-xs text-muted-foreground">
            这个工作树已不在仓库中，可能已被删除。
          </p>
        {/if}
      </section>
    {/if}
  </div>
</ResponsiveDialog>
