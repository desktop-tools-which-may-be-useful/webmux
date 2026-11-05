<script lang="ts">
  import IconMenu2 from "~icons/tabler/menu-2";
  import { Button } from "$lib/components/ui/button";
  import SessionFilters from "../../src/components/SessionFilters.svelte";
  import WorkspaceGroup from "../../src/components/WorkspaceGroup.svelte";
  import WorkspaceToolbar from "../../src/components/WorkspaceToolbar.svelte";
  import type { StatusFilter } from "../../src/session-status";
  import { now } from "../fixtures";
  import { directEntries, repositories, sessionInfo } from "./data";

  // Prototype of the workbench main area: the existing overview (toolbar,
  // filters, worktree groups) unchanged, minus the home header and the
  // repository list that moved into the sidebar. A worktree picked in the
  // tree scrolls into view.
  let {
    scope,
    worktree,
    openDrawer,
  }: { scope: string; worktree?: string; openDrawer: () => void } = $props();
  let query = $state("");
  let statusFilter = $state<StatusFilter>("all");
  let notice = $state("");
  const shown = $derived(
    scope === "all"
      ? repositories.filter((repo) => !repo.reason)
      : repositories.filter((repo) => repo.id === scope),
  );
  const groups = $derived(
    shown.flatMap((repo) =>
      repo.worktrees.map((tree) => ({
        id: tree.id,
        title: scope === "all" ? `${repo.name} · ${tree.title}` : tree.title,
        path: tree.path,
        sessions: tree.entries.map((item) => sessionInfo(item, tree.path)),
      })),
    ),
  );
  const direct = $derived(
    directEntries.map((item) => sessionInfo(item, "/home/developer")),
  );
  const sessions = $derived(
    scope === "direct" ? direct : groups.flatMap((group) => group.sessions),
  );
  const title = $derived(
    scope === "all"
      ? "全部仓库"
      : scope === "direct"
        ? "直接会话"
        : (shown[0]?.name ?? "工作区"),
  );
  $effect(() => {
    if (!worktree) return;
    document
      .getElementById(`wt-${worktree}`)
      ?.scrollIntoView({ block: "start" });
  });
  const sample = () => (notice = "此预览使用示例数据，未执行真实操作。");
</script>

<div class="h-full overflow-y-auto">
  <header class="flex h-12 items-center gap-1 border-b px-2 sm:hidden">
    <Button
      variant="ghost"
      size="icon-touch"
      aria-label="打开导航"
      title="打开导航"
      onclick={openDrawer}><IconMenu2 aria-hidden="true" /></Button
    >
    <h1 class="text-sm font-medium">{title}</h1>
  </header>
  <main class="mx-auto max-w-6xl px-4 py-5 sm:px-8 sm:py-6">
    <WorkspaceToolbar
      {title}
      loading={false}
      filtering={false}
      visibleCount={sessions.length}
      sessionCount={sessions.length}
      repositoryCount={scope === "all" ? shown.length : 1}
      canAddRepository
      refreshing={false}
      onadd={sample}
      onrefresh={sample}
      onstart={sample}
    >
      <SessionFilters
        {sessions}
        archivedCount={4}
        bind:query
        bind:statusFilter
        filtered={false}
        shortcutDisabled={false}
        onclear={() => {
          query = "";
          statusFilter = "all";
        }}
      />
    </WorkspaceToolbar>
    {#if notice}<p role="status" class="mb-4 text-sm text-muted-foreground">
        {notice}
      </p>{/if}
    <div class="space-y-6">
      {#if scope === "direct"}
        <WorkspaceGroup
          title="直接会话"
          path="/home/developer"
          sessions={direct}
          {now}
          onclose={sample}
        />
      {:else}
        {#each groups as group (group.id)}
          <section id={`wt-${group.id}`} class="scroll-mt-4">
            <WorkspaceGroup
              title={group.title}
              path={group.path}
              sessions={group.sessions}
              {now}
              onclose={sample}
              onstart={sample}
            />
          </section>
        {/each}
      {/if}
    </div>
  </main>
</div>
