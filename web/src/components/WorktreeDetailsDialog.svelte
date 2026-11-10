<script lang="ts">
  import type { SessionInfo, Worktree } from "../schemas";
  import ResponsiveDialog from "./ResponsiveDialog.svelte";
  import WorktreeDetails from "./WorktreeDetails.svelte";

  // The dialog holding a worktree's (or directory's) details outside the
  // repository panel: the home page's groups and the sidebar.
  let {
    title,
    path,
    worktree,
    sessions,
    lastFetchedAt,
    onclose,
  }: {
    title: string;
    path?: string;
    worktree?: Worktree;
    sessions: readonly SessionInfo[];
    lastFetchedAt?: string | null;
    onclose: () => void;
  } = $props();
</script>

<ResponsiveDialog
  {title}
  description={worktree
    ? "工作树路径、Git 同步状态与占用空间。"
    : "目录与会话。"}
  {onclose}
>
  <WorktreeDetails {path} {worktree} {sessions} {lastFetchedAt} />
</ResponsiveDialog>
