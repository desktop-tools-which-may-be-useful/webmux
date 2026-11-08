<script lang="ts">
  import IconCopy from "~icons/tabler/copy";
  import { Button } from "$lib/components/ui/button";
  import { copyText } from "../clipboard";
  import { placeArchived } from "../archived";
  import { homeLayout } from "../home-worktrees";
  import type {
    ArchivedSession,
    HomeWorktrees,
    Repository,
    SessionInfo,
  } from "../schemas";
  import { statusSummary } from "../session-status";
  import ResponsiveDialog from "./ResponsiveDialog.svelte";

  // Where a repository lives and what its home page shows: path, how many
  // worktrees are listed, the session summary and the Git read time. Removing
  // the registration starts here.
  let {
    repository,
    sessions,
    records,
    home,
    refreshing,
    registryReady,
    onunregister,
    onclose,
  }: {
    repository: Repository;
    sessions: readonly SessionInfo[];
    records: readonly ArchivedSession[];
    home: HomeWorktrees;
    refreshing: boolean;
    registryReady: boolean;
    onunregister: () => void;
    onclose: () => void;
  } = $props();
  const own = $derived(
    sessions.filter((s) => s.workspace?.repositoryId === repository.id),
  );
  const shown = $derived(
    homeLayout(
      home,
      repository,
      own,
      placeArchived(
        records.filter((record) => !sessions.some((s) => s.id === record.id)),
        [repository],
      ).repositories.get(repository.id)?.worktrees,
    ).shown.length,
  );
</script>

<ResponsiveDialog
  title={repository.name}
  description="仓库路径、主页显示的工作树与 Git 状态。"
  {onclose}
>
  <div class="space-y-3">
    <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-xs">
      <dt class="text-muted-foreground">路径</dt>
      <dd class="flex min-w-0 items-start gap-1">
        <span class="min-w-0 flex-1 font-mono break-all"
          >{repository.anchorPath}</span
        >
        <Button
          size="icon-xs"
          variant="ghost"
          aria-label="复制仓库路径"
          title="复制仓库路径"
          onclick={() => void copyText(repository.anchorPath)}
          ><IconCopy aria-hidden="true" /></Button
        >
      </dd>
      <dt class="text-muted-foreground">工作区</dt>
      <dd>主页 {shown} / 共 {repository.worktrees.length}</dd>
      <dt class="text-muted-foreground">会话</dt>
      <dd>{statusSummary(own)}</dd>
      <dt class="text-muted-foreground">Git</dt>
      <dd>
        {refreshing
          ? "正在重新读取状态…"
          : `读取于 ${new Date(repository.checkedAt).toLocaleString()}`}{repository.availability ===
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
          // The parent reads the repository from the open dialog's target.
          onunregister();
          onclose();
        }}>从列表移除仓库…</Button
      >
    </div>
  </div>
</ResponsiveDialog>
