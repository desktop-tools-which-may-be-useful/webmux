<script lang="ts">
  import { errorMessage } from "../error";
  import { api } from "../api";
  import { NoContentSchema, type SessionInfo } from "../schemas";
  import { Button } from "$lib/components/ui/button";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";

  // Ends a session. A live hybrid session is archived instead: its agent
  // stops, the conversation stays with the agent and the record can be
  // resumed from its worktree's archived section.
  let {
    session,
    onclose,
    onremoved,
    onarchived,
  }: {
    session: SessionInfo;
    onclose: () => void;
    onremoved?: () => void;
    onarchived?: (session: SessionInfo) => void;
  } = $props();
  let open = $state(true);
  let busy = $state(false);
  let error = $state("");
  const removing = $derived(session.status === "exited");
  const archiving = $derived(
    !removing && session.views.includes("chat") && session.state === "active",
  );

  async function submit() {
    busy = true;
    error = "";
    try {
      await api(
        `/sessions/${session.id}${removing ? "?remove=1" : ""}`,
        NoContentSchema,
        "DELETE",
      );
      if (removing) onremoved?.();
      if (archiving) onarchived?.(session);
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<AlertDialog.Root
  bind:open
  onOpenChange={(value) => {
    if (!value && !busy) onclose();
  }}
>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title
        >{removing
          ? "移除"
          : archiving
            ? "停止并归档"
            : "关闭"}“{session.title}”？</AlertDialog.Title
      >
      <AlertDialog.Description>
        {removing
          ? "会话及其缓冲将永久移除。"
          : archiving
            ? "agent 进程将停止，对话保留在 agent 中。归档后可在所属工作树的“已归档”中恢复到聊天或终端。"
            : "正在运行的进程及子进程将终止。最后输出会保留，直到手动移除。"}
      </AlertDialog.Description>
    </AlertDialog.Header>
    {#if error}<p role="alert" class="text-destructive">{error}</p>{/if}
    <AlertDialog.Footer>
      <AlertDialog.Cancel disabled={busy}>取消</AlertDialog.Cancel>
      <Button
        variant={archiving ? "default" : "destructive"}
        disabled={busy}
        onclick={submit}
      >
        {busy
          ? "正在处理…"
          : removing
            ? "确认移除"
            : archiving
              ? "停止并归档"
              : "确认关闭"}
      </Button>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
