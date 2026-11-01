<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import ConnectorInstallHint from "./ConnectorInstallHint.svelte";
  import HybridCandidatePicker from "./HybridCandidatePicker.svelte";
  import { errorMessage } from "../error";
  import type { ResumeChoice, ResumeOutcome } from "../recoverable";
  import type {
    AgentConnector,
    HybridCandidate,
    HybridRecord,
    SessionInfo,
  } from "../schemas";

  // "恢复到聊天 / 恢复到终端" for one closed or interrupted hybrid session,
  // with the questions a resume may need answered first: another agent
  // session when the recorded one is gone, or resuming without a deleted
  // launch config's environment.
  let {
    record,
    resume,
    onresumed,
    disabled = false,
    onbusy = () => {},
    onerror,
  }: {
    record: HybridRecord;
    resume: (
      id: string,
      side: "tui" | "acp",
      choice?: ResumeChoice,
    ) => Promise<ResumeOutcome>;
    onresumed: (session: SessionInfo) => void;
    disabled?: boolean;
    onbusy?: (busy: boolean) => void;
    /** Reports a failed resume; "" clears it when a new attempt starts. */
    onerror: (message: string) => void;
  } = $props();

  // A resume that needs the user's answer before it can be retried. Earlier
  // answers ride along, so a second question does not undo the first.
  let pending = $state<{
    side: "tui" | "acp";
    choice: ResumeChoice;
    kind: "candidates" | "config_missing";
    message: string;
    candidates: HybridCandidate[];
  } | null>(null);
  let pendingOpen = $state(false);
  let picked = $state("");

  async function run(side: "tui" | "acp", choice: ResumeChoice = {}) {
    onbusy(true);
    onerror("");
    try {
      const outcome = await resume(record.id, side, choice);
      if (outcome.status === "resumed") {
        onresumed(outcome.session);
        return;
      }
      if (outcome.status === "connector_missing") {
        missing = { connector: outcome.connector, choice };
        missingOpen = true;
        return;
      }
      pending = {
        side,
        choice,
        kind: outcome.status,
        message: outcome.message,
        candidates: outcome.status === "candidates" ? outcome.candidates : [],
      };
      picked = pending.candidates[0]?.agentSessionId ?? "";
      pendingOpen = true;
    } catch (err) {
      onerror(`无法恢复“${record.name}”：${errorMessage(err)}`);
    } finally {
      onbusy(false);
    }
  }

  function confirmPending() {
    const answer = pending;
    pendingOpen = false;
    if (!answer) return;
    void run(
      answer.side,
      answer.kind === "candidates"
        ? { ...answer.choice, agentSessionId: picked }
        : { ...answer.choice, confirmWithoutEnv: true },
    );
  }

  // The chat side's connector is not installed; the terminal side still
  // works, with the answers given so far.
  let missing = $state<{
    connector: AgentConnector;
    choice: ResumeChoice;
  } | null>(null);
  let missingOpen = $state(false);

  function resumeTerminal() {
    const answer = missing;
    missingOpen = false;
    if (answer) void run("tui", answer.choice);
  }
</script>

<Button size="sm" {disabled} onclick={() => run("acp")}>恢复到聊天</Button>
<Button size="sm" variant="outline" {disabled} onclick={() => run("tui")}
  >恢复到终端</Button
>

<ConfirmDialog
  bind:open={pendingOpen}
  title={pending?.kind === "candidates"
    ? "找不到原来的 agent 会话"
    : "启动配置已删除"}
  onconfirm={confirmPending}
>
  {pending?.message}
  {#if pending?.kind === "candidates"}
    <HybridCandidatePicker
      candidates={pending.candidates}
      bind:picked
      label="选择要恢复的会话"
    />
  {:else}
    恢复后将使用原命令，但不再带有该配置保存的环境变量。
  {/if}
</ConfirmDialog>

<AlertDialog.Root bind:open={missingOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>聊天端无法启动</AlertDialog.Title>
      <AlertDialog.Description
        >可以先安装后再恢复到聊天，或者恢复到终端。</AlertDialog.Description
      >
    </AlertDialog.Header>
    {#if missing}<ConnectorInstallHint connector={missing.connector} />{/if}
    <AlertDialog.Footer>
      <AlertDialog.Cancel>取消</AlertDialog.Cancel>
      <AlertDialog.Action onclick={resumeTerminal}
        >恢复到终端</AlertDialog.Action
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
