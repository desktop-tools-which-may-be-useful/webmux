<script lang="ts">
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconCircleCheck from "~icons/tabler/circle-check";
  import IconHandStop from "~icons/tabler/hand-stop";
  import IconLoader2 from "~icons/tabler/loader-2";
  import IconX from "~icons/tabler/x";
  import { Button } from "$lib/components/ui/button";
  import InlineNotice from "../InlineNotice.svelte";
  import ConnectorInstallHint from "../ConnectorInstallHint.svelte";
  import { outdatedText } from "../../hybrid";
  import type { ChatActions, RequestAnswer } from "../../chat-api";
  import { groupTimeline, type ChatModel } from "../../chat-model";
  import { errorMessage } from "../../error";
  import type { ChatUploads } from "../../hooks/upload.svelte";
  import type { ChatState } from "../../schemas";
  import type {
    TerminalDraft,
    TerminalDrafts,
  } from "../../terminal-draft.svelte";
  import type { Connection } from "../../ws";
  import ChatComposer from "./ChatComposer.svelte";
  import ChatPlan from "./ChatPlan.svelte";
  import ChatQueue from "./ChatQueue.svelte";
  import FormPrompt from "./FormPrompt.svelte";
  import PermissionPrompt from "./PermissionPrompt.svelte";
  import TimelineItem from "./TimelineItem.svelte";

  // The chat side of a hybrid session, in the pane the terminal fills
  // otherwise: the transcript, then a dock with the plan, the queue, a
  // pending request or the input.
  let {
    model,
    chat,
    connection,
    draft,
    drafts,
    uploads,
    actions,
    unavailable = "",
    uploadUnavailable = "",
    onreview,
    onterminal,
  }: {
    model: ChatModel;
    chat: ChatState | undefined;
    connection: Connection;
    draft: TerminalDraft;
    drafts: TerminalDrafts;
    uploads: ChatUploads;
    actions: ChatActions;
    /** Why the page holds input back (a switch in progress); "" otherwise. */
    unavailable?: string;
    /** Why uploads are refused (session gone or exited); "" otherwise. */
    uploadUnavailable?: string;
    /** Opens a changed file in the code panel. */
    onreview: (path: string) => void;
    /** Moves the session to its terminal, such as to log in there. */
    onterminal: () => void;
  } = $props();

  let scroller = $state<HTMLDivElement>();
  let stick = $state(true);
  let error = $state("");
  let answering = $state(false);
  let dragging = $state(false);
  let now = $state(Date.now());
  let dragDepth = 0;
  let composer = $state<ChatComposer>();
  export function focus() {
    composer?.focus();
  }
  const entries = $derived(groupTimeline(model.entries));
  const request = $derived(chat?.requests[0]);
  const requestTool = $derived.by(() => {
    const at = request?.toolCallId
      ? model.index.get(`tool:${request.toolCallId}`)
      : undefined;
    const entry = at === undefined ? undefined : model.entries[at];
    return entry?.type === "tool" ? entry : undefined;
  });
  const waiting = $derived(
    (chat?.requests ?? []).flatMap((item) =>
      item.toolCallId ? [item.toolCallId] : [],
    ),
  );
  const blocked = $derived(
    unavailable ||
      (chat?.recovery
        ? "会话已中断，请先恢复"
        : connection !== "connected" || !chat
          ? "正在连接聊天…"
          : chat.side !== "acp"
            ? "会话不在聊天端"
            : ""),
  );
  const uploadBlocked = $derived(blocked || uploadUnavailable);
  const turnStartedAt = $derived(
    chat?.turn.startedAt ? Date.parse(chat.turn.startedAt) : undefined,
  );
  const turnEndedAt = $derived(
    chat?.turn.endedAt ? Date.parse(chat.turn.endedAt) : undefined,
  );
  const turnElapsed = $derived.by(() => {
    if (turnStartedAt === undefined) return undefined;
    const end = chat?.turn.running ? now : turnEndedAt;
    return end === undefined ? undefined : Math.max(0, end - turnStartedAt);
  });

  function formatDuration(milliseconds: number) {
    const totalSeconds = Math.floor(milliseconds / 1000);
    const seconds = totalSeconds % 60;
    const totalMinutes = Math.floor(totalSeconds / 60);
    const minutes = totalMinutes % 60;
    const hours = Math.floor(totalMinutes / 60);
    if (hours)
      return `${hours} 小时 ${String(minutes).padStart(2, "0")} 分 ${String(seconds).padStart(2, "0")} 秒`;
    if (totalMinutes)
      return `${totalMinutes} 分 ${String(seconds).padStart(2, "0")} 秒`;
    return `${seconds} 秒`;
  }

  const turnCompletionLabel = $derived(
    chat?.turn.stopReason === "end_turn"
      ? "已完成"
      : chat?.turn.stopReason === "cancelled"
        ? "已中断"
        : "已结束",
  );
  const turnFailed = $derived(
    chat?.turn.stopReason === "error" ||
      chat?.turn.stopReason === "refusal" ||
      chat?.turn.stopReason === "auth_required",
  );
  const turnCompletionAnnouncement = $derived.by(() => {
    if (
      !chat ||
      chat.turn.running ||
      turnElapsed === undefined ||
      turnEndedAt === undefined
    )
      return "";
    return `${turnCompletionLabel} · 用时 ${formatDuration(turnElapsed)}`;
  });

  // Server timestamps keep the duration correct across reconnects and devices.
  // Only the live display ticks; the total is derived from the final timestamp.
  $effect(() => {
    if (!chat?.turn.running || turnStartedAt === undefined) return;
    now = Date.now();
    const timer = window.setInterval(() => (now = Date.now()), 1000);
    return () => window.clearInterval(timer);
  });

  const hasFiles = (event: DragEvent) =>
    event.dataTransfer?.types.includes("Files") ?? false;
  function dragEnter(event: DragEvent) {
    if (!hasFiles(event)) return;
    event.preventDefault();
    if (uploadBlocked) return;
    dragging = true;
    dragDepth++;
  }
  function dragOver(event: DragEvent) {
    if (!hasFiles(event)) return;
    event.preventDefault();
    if (event.dataTransfer)
      event.dataTransfer.dropEffect = uploadBlocked
        ? "none"
        : uploads.progress
          ? "none"
          : "copy";
  }
  function dragLeave() {
    if (--dragDepth <= 0) {
      dragDepth = 0;
      dragging = false;
    }
  }
  function drop(event: DragEvent) {
    if (!hasFiles(event)) return;
    event.preventDefault();
    dragDepth = 0;
    dragging = false;
    if (uploadBlocked) return;
    void uploads.uploadFiles(Array.from(event.dataTransfer?.files ?? []));
  }

  function follow() {
    if (!scroller) return;
    stick =
      scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - 32;
  }
  function latest() {
    scroller?.scrollTo({ top: scroller.scrollHeight });
    stick = true;
  }
  // New output keeps the newest entry in view unless the reader scrolled up.
  $effect(() => {
    void entries;
    void request;
    void chat?.turn.running;
    if (stick && scroller) scroller.scrollTop = scroller.scrollHeight;
  });

  function answer(requestId: string, reply: RequestAnswer) {
    answering = true;
    actions
      .answer(requestId, reply)
      .then(
        () => (error = ""),
        (err: unknown) => (error = errorMessage(err)),
      )
      .finally(() => (answering = false));
  }
</script>

<div
  class="relative flex h-full min-h-0 flex-col bg-background"
  data-chat-view
  role="region"
  aria-label="聊天"
  ondragenter={dragEnter}
  ondragover={dragOver}
  ondragleave={dragLeave}
  ondrop={drop}
>
  {#if dragging}
    <div
      class="pointer-events-none absolute inset-2 z-20 flex items-center justify-center rounded-lg border-2 border-dashed border-ring bg-background/85 text-sm"
      role="status"
    >
      {uploads.progress ? "请等待当前上传完成" : "松开以上传文件到聊天"}
    </div>
  {/if}
  <div class="relative flex min-h-0 flex-1 flex-col">
    <div
      bind:this={scroller}
      class="min-h-0 flex-1 overflow-y-auto"
      onscroll={follow}
    >
      <div class="mx-auto flex max-w-3xl flex-col gap-2.5 px-4 py-6">
        {#if model.truncated}
          <InlineNotice>更早的历史未缓存，可切到终端查看</InlineNotice>
        {/if}
        {#each chat?.outdated ?? [] as item (item.name)}
          <InlineNotice tone="warning">{outdatedText(item)}</InlineNotice>
        {/each}
        {#if chat?.unsupported.length}
          <InlineNotice tone="warning"
            >聊天视图不支持该参数：{chat.unsupported.join(" ")}</InlineNotice
          >
        {/if}
        <ol class="flex flex-col gap-2.5" aria-label="对话">
          {#each entries as entry (`${entry.type}:${entry.id}`)}
            <li class="min-w-0">
              <TimelineItem {entry} {waiting} {onreview} />
            </li>
          {/each}
        </ol>
        <!-- A persistent live region announces only the final duration. The
             visible per-second ticker stays out of every live region. -->
        <p class="sr-only" role="status">{turnCompletionAnnouncement}</p>
        {#if !chat}
          <p
            class="flex items-center gap-2 text-sm text-muted-foreground"
            role="status"
          >
            <IconLoader2 class="size-3.5 animate-spin" aria-hidden="true" />
            正在连接聊天…
          </p>
        {:else if chat.turn.running && !request}
          <p
            class="flex items-center gap-2 text-sm text-muted-foreground"
            role="status"
          >
            <IconLoader2 class="size-3.5 animate-spin" aria-hidden="true" />
            正在工作…
            {#if turnElapsed !== undefined}
              <span class="tabular-nums" aria-hidden="true"
                >· {formatDuration(turnElapsed)}</span
              >
            {/if}
          </p>
        {:else if turnElapsed !== undefined && turnEndedAt !== undefined}
          <p
            class="flex items-center gap-2 text-sm text-muted-foreground"
            aria-hidden="true"
            data-turn-result={chat.turn.stopReason}
          >
            {#if chat.turn.stopReason === "end_turn"}
              <IconCircleCheck
                class="size-3.5 text-success"
                aria-hidden="true"
              />
            {:else if chat.turn.stopReason === "cancelled"}
              <IconHandStop class="size-3.5 text-warning" aria-hidden="true" />
            {:else if turnFailed}
              <IconX class="size-3.5 text-destructive" aria-hidden="true" />
            {:else}
              <IconCircleCheck class="size-3.5" aria-hidden="true" />
            {/if}
            {turnCompletionLabel}
            <span class="tabular-nums"
              >· 用时 {formatDuration(turnElapsed)}</span
            >
          </p>
        {/if}
      </div>
    </div>
    {#if !stick}
      <!-- Anchored to the message area, so a taller composer never covers it. -->
      <Button
        variant="outline"
        size="sm"
        raised
        class="absolute bottom-3 left-1/2 z-10 -translate-x-1/2"
        onclick={latest}><IconArrowDown aria-hidden="true" />回到最新</Button
      >
    {/if}
  </div>
  <div class="mx-auto w-full max-w-3xl space-y-2 px-3 pb-safe-4 sm:px-4">
    {#if error}<InlineNotice tone="danger">{error}</InlineNotice>{/if}
    {#if chat?.connectorMissing}
      <ConnectorInstallHint connector={chat.connectorMissing}>
        {#snippet actions()}<Button
            size="sm"
            variant="outline"
            disabled={!!unavailable}
            onclick={onterminal}>切到终端</Button
          >{/snippet}
      </ConnectorInstallHint>
    {/if}
    {#if chat?.authRequired}
      <InlineNotice tone="danger">
        <span class="flex flex-wrap items-center gap-2"
          >Agent 需要登录，请在终端中完成登录。<Button
            size="sm"
            variant="outline"
            disabled={!!unavailable}
            onclick={onterminal}>切到终端登录</Button
          ></span
        >
      </InlineNotice>
    {/if}
    {#if chat?.plan?.entries.length}<ChatPlan plan={chat.plan} />{/if}
    {#if chat?.queue.items.length}
      <ChatQueue
        queue={chat.queue}
        running={chat.turn.running}
        disabled={!!blocked}
        {actions}
        onerror={(message) => (error = message)}
      />
    {/if}
    {#if request?.kind === "permission"}
      <PermissionPrompt
        {request}
        tool={requestTool}
        disabled={!!blocked || answering}
        onanswer={(reply) => answer(request.requestId, reply)}
      />
    {:else if request?.kind === "form"}
      {#key request.requestId}
        <FormPrompt
          {request}
          disabled={!!blocked || answering}
          onanswer={(reply) => answer(request.requestId, reply)}
        />
      {/key}
    {:else if chat}
      <ChatComposer
        bind:this={composer}
        {draft}
        {drafts}
        {chat}
        {uploads}
        unavailable={blocked}
        {uploadUnavailable}
        {actions}
        onerror={(message) => (error = message)}
      />
    {/if}
  </div>
</div>
