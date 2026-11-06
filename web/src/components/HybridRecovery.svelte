<script lang="ts">
  import { findShell, shellUi } from "../shell.svelte";
  import { Button } from "$lib/components/ui/button";
  import { navigate } from "../api";
  import { resumeSession } from "../recoverable";
  import type { ArchivedSession } from "../schemas";
  import HybridRecordHints from "./HybridRecordHints.svelte";
  import HybridResumeActions from "./HybridResumeActions.svelte";

  // The session page of a closed or interrupted hybrid session: the agent
  // still has the conversation, so it can come back on either side.
  let {
    record,
    onresumed,
  }: {
    record: ArchivedSession;
    onresumed: () => void;
  } = $props();
  let busy = $state(false);
  let error = $state("");
  const shell = findShell();
  // A rebuilt worktree changes the repository's worktrees and the records,
  // whether or not the resume that follows succeeds; this page stays, so a
  // failure shows here like any other.
  function rebuilt(failure: string) {
    if (failure) error = failure;
    void shell?.repositories.refresh();
    void shell?.archived.refresh();
  }
</script>

<div
  class="flex shrink-0 flex-col gap-2 border-b bg-muted px-4 py-2 text-sm"
  role="status"
  aria-label="恢复会话"
>
  <p>
    {record.state === "closed"
      ? "该混合会话已关闭"
      : "该混合会话已中断"}，对话仍保存在 {record.agent.name} 中，可以恢复到聊天或终端。
  </p>
  <HybridRecordHints {record} />
  {#if error}<p class="text-destructive" role="alert">{error}</p>{/if}
  <div class="flex flex-wrap gap-2">
    <HybridResumeActions
      {record}
      resume={resumeSession}
      onresumed={() => onresumed()}
      onrebuilt={rebuilt}
      disabled={busy}
      onbusy={(active) => (busy = active)}
      onerror={(message) => (error = message)}
    />
    <Button variant="ghost" size="sm" onclick={() => navigate(shellUi.home)}
      >返回列表</Button
    >
  </div>
</div>
