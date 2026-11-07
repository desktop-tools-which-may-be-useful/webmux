<script lang="ts">
  import type { Snippet } from "svelte";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconPlayerStopFilled from "~icons/tabler/player-stop-filled";
  import { Button, type ButtonSize } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { Textarea } from "$lib/components/ui/textarea";
  import type { ChatActions, ConfigValue } from "../../chat-api";
  import { errorMessage } from "../../error";
  import { useComposerCompact } from "../../hooks/composer-compact.svelte";
  import { useTextareaDraft } from "../../hooks/textarea-draft.svelte";
  import type { ChatUploads } from "../../hooks/upload.svelte";
  import type {
    ChatCommand,
    ChatPermissionMode as PermissionMode,
    ChatState,
    SessionInfo,
  } from "../../schemas";
  import type {
    TerminalDraft,
    TerminalDrafts,
  } from "../../terminal-draft.svelte";
  import { hasTouchInput } from "../../touch-input";
  import FileUploadProgress from "../FileUploadProgress.svelte";
  import ChatAttachmentList from "./ChatAttachmentList.svelte";
  import ChatConfig from "./ChatConfig.svelte";
  import ChatLocation from "./ChatLocation.svelte";
  import ChatPermissionMode from "./ChatPermissionMode.svelte";
  import ChatSessionOptions from "./ChatSessionOptions.svelte";
  import ChatUploadButton from "./ChatUploadButton.svelte";
  import UsageRing from "./UsageRing.svelte";

  // The chat input. Its text is the session's draft, saved on this device
  // like the terminal draft. Sending while a turn runs queues on the server;
  // Ctrl/Cmd+Enter injects into the running turn when the agent takes it, and
  // the send button stops the turn while there is nothing to send.
  // "/" opens the agent's commands.
  let {
    draft,
    drafts,
    chat,
    unavailable,
    uploads,
    uploadUnavailable,
    location,
    actions,
    onerror,
    quickMenu,
  }: {
    draft: TerminalDraft;
    drafts: TerminalDrafts;
    chat: ChatState;
    /** Why input is disabled now (switching, offline); "" when enabled. */
    unavailable: string;
    uploads: ChatUploads;
    /** Why uploads are refused (session gone or exited); "" otherwise. */
    uploadUnavailable: string;
    /** Where the session runs, shown read-only above the input. */
    location?: Pick<SessionInfo, "workspace" | "cwd">;
    actions: ChatActions;
    onerror: (message: string) => void;
    quickMenu?: Snippet<["top" | "bottom", ButtonSize?]>;
  } = $props();

  let textarea = $state<HTMLTextAreaElement | null>(null);
  let container = $state<HTMLDivElement | null>(null);
  let sending = $state(false);
  const selection = useTextareaDraft(
    () => textarea,
    () => draft,
    () => drafts,
  );
  // A permission mode change is on its way; the select waits for it.
  let savingMode = $state(false);
  let highlighted = $state(0);
  // Escape hides the menu for the text it was open on.
  let menuDismissed = $state<string | null>(null);
  const running = $derived(chat.turn.running);
  const typed = $derived(draft.text.trim() !== "");
  const ready = $derived(typed || uploads.attachments.length > 0);
  const imageDegraded = $derived(
    !chat.promptCapabilities.image &&
      uploads.attachments.some((item) => item.kind === "image"),
  );
  const locked = $derived(!!draft.submission);
  const disabled = $derived(!!unavailable || locked);
  const uploading = $derived(uploads.progress !== null);
  const compactState = useComposerCompact(
    () => container,
    () =>
      Boolean(
        draft.text || uploads.attachments.length > 0 || uploading || sending,
      ),
  );
  const compact = $derived(compactState.compact);
  const steerable = $derived(running && chat.steering);
  // With nothing to send the send button stops the running turn.
  const stopping = $derived(running && !ready && !uploading && !sending);
  const queueing = $derived(running || chat.queue.paused);
  const query = $derived(
    /^\/\S*$/.test(draft.text) ? draft.text.slice(1).toLowerCase() : null,
  );
  const matches = $derived(
    query === null
      ? []
      : chat.commands.filter((command) =>
          command.name.toLowerCase().startsWith(query),
        ),
  );
  const menuOpen = $derived(
    !disabled && matches.length > 0 && menuDismissed !== draft.text,
  );
  const active = $derived(Math.min(highlighted, matches.length - 1));

  async function submit(mode: "auto" | "steer") {
    const text = draft.text.trim();
    if (!ready || sending || disabled || uploading) return;
    sending = true;
    drafts.edit(draft, "", 0);
    const sent = uploads.take();
    try {
      await actions.send(text, mode, sent);
      onerror("");
    } catch (err) {
      // Give the text and attachments back; keep anything typed since.
      if (!draft.text) drafts.edit(draft, text, text.length);
      uploads.restore(sent);
      onerror(errorMessage(err));
    } finally {
      sending = false;
    }
  }
  function pick(command: ChatCommand) {
    const text = `/${command.name} `;
    drafts.edit(draft, text, text.length);
    textarea?.focus();
  }
  function keydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (menuOpen) {
      const command = matches[active];
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const step = event.key === "ArrowDown" ? 1 : -1;
        highlighted = (active + step + matches.length) % matches.length;
        return;
      }
      if (event.key === "Tab" && command) {
        event.preventDefault();
        pick(command);
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        menuDismissed = draft.text;
        return;
      }
    }
    if (event.key !== "Enter" || event.shiftKey) return;
    const steer = event.ctrlKey || event.metaKey;
    // On touch keyboards Enter starts a new line; the button sends.
    if (!steer && hasTouchInput()) return;
    event.preventDefault();
    void submit(steer && steerable ? "steer" : "auto");
  }
  async function stop() {
    try {
      await actions.cancel();
    } catch (err) {
      onerror(errorMessage(err));
    }
  }
  async function setPermissionMode(mode: PermissionMode) {
    savingMode = true;
    try {
      await actions.setPermissionMode(mode);
      onerror("");
    } catch (err) {
      onerror(errorMessage(err));
    } finally {
      savingMode = false;
    }
  }
  function setConfig(configId: string, value: ConfigValue) {
    actions.setConfig(configId, value).then(
      () => onerror(""),
      (err: unknown) => onerror(errorMessage(err)),
    );
  }
  export function focus() {
    textarea?.focus();
  }
</script>

<div
  bind:this={container}
  class="space-y-2 sm:space-y-1.5"
  data-chat-composer
  data-compact={compact ? "" : undefined}
  {...compactState.containerProps}
>
  {#if location}
    <ChatLocation
      workspace={location.workspace}
      cwd={location.cwd}
      class={compact ? "max-sm:hidden" : ""}
    />
  {/if}
  <!-- The card shows the message box's focus once; the box itself has none. -->
  <div
    class={[
      "rounded-lg border bg-card shadow-sm transition has-[textarea:focus]:border-ring has-[textarea:focus]:ring-3 has-[textarea:focus]:ring-ring/20",
      compact && "max-sm:flex max-sm:items-center max-sm:gap-1 max-sm:p-0.5",
    ]}
  >
    {#if menuOpen}
      <div
        id="chat-commands"
        class="max-h-52 overflow-y-auto border-b p-1"
        role="listbox"
        aria-label="命令"
      >
        {#each matches as command, i (command.name)}
          <ListButton
            variant="suggestion"
            role="option"
            aria-selected={i === active}
            class="flex w-full min-w-0 flex-wrap items-baseline"
            onmousedown={(event) => event.preventDefault()}
            onclick={() => pick(command)}
          >
            <span class="shrink-0 font-mono text-sm">/{command.name}</span>
            {#if command.hint}<span
                class="min-w-0 break-words text-muted-foreground"
                >&lt;{command.hint}&gt;</span
              >{/if}
            <span class="ml-auto min-w-0 truncate text-muted-foreground"
              >{command.description}</span
            >
          </ListButton>
        {/each}
      </div>
    {/if}
    <ChatAttachmentList {uploads} disabled={sending} />
    {#if imageDegraded}
      <p class="border-b px-3 py-1.5 text-xs text-warning" role="status">
        该 agent 不支持图片，图片将按服务器路径发送。
      </p>
    {/if}
    <div
      class={[
        "px-2 pt-1",
        compact && "max-sm:min-w-0 max-sm:flex-1 max-sm:px-1 max-sm:py-0",
      ]}
    >
      <Textarea
        bind:ref={textarea}
        variant="bare"
        onpaste={(event) => uploads.filePaste(event)}
        rows={compact ? 1 : 2}
        class={[
          "max-h-48 min-h-14 resize-none",
          compact && "max-sm:max-h-9 max-sm:min-h-9 max-sm:overflow-hidden",
        ]}
        placeholder={unavailable ||
          (locked
            ? "草稿正在提交到终端，确认结果前不能编辑"
            : queueing
              ? "继续输入，发送后加入排队"
              : "给 Agent 发消息，输入 / 查看命令")}
        aria-label="消息"
        aria-controls={menuOpen ? "chat-commands" : undefined}
        aria-expanded={menuOpen}
        {disabled}
        value={draft.text}
        oninput={(event) => {
          const input = event.currentTarget;
          drafts.edit(
            draft,
            input.value,
            input.selectionStart,
            input.selectionEnd,
          );
          highlighted = 0;
        }}
        {...selection.selectionProps}
        onkeydown={keydown}
      />
    </div>
    {#if uploads.progress}<FileUploadProgress
        progress={uploads.progress}
        cancel={() => uploads.cancel()}
        variant="card"
      />{/if}
    <!-- Only what acts on the message stays in the card: context use and the
       send controls (they wrap only at very large text). -->
    <div
      class={[
        "flex flex-wrap items-center justify-end gap-1 px-2 pb-2",
        compact && "max-sm:shrink-0 max-sm:p-0",
      ]}
    >
      <div
        class={["mr-auto flex items-center gap-1", compact && "max-sm:hidden"]}
      >
        <ChatUploadButton
          {uploads}
          {disabled}
          unavailable={uploadUnavailable}
        />
        <span class="sm:hidden">
          <ChatSessionOptions
            mode={chat.permissionMode}
            config={chat.config}
            {disabled}
            modeDisabled={!!unavailable || savingMode}
            onmodechange={(mode) => void setPermissionMode(mode)}
            onconfigchange={setConfig}
          />
        </span>
      </div>
      {#if chat.usage}<span class={["contents", compact && "max-sm:hidden"]}
          ><UsageRing usage={chat.usage} /></span
        >{/if}
      <span class={["flex items-center gap-1", compact && "max-sm:hidden"]}>
        {@render quickMenu?.("top", "icon")}
      </span>
      {#if stopping}
        <Button
          size="icon"
          aria-label="停止"
          title="停止当前回合"
          disabled={!!unavailable}
          onclick={stop}><IconPlayerStopFilled aria-hidden="true" /></Button
        >
      {:else}
        <Button
          size="icon"
          aria-label={queueing ? "加入排队" : "发送"}
          title={queueing ? "加入排队" : "发送"}
          disabled={disabled || !ready || sending || uploading}
          onclick={() => submit("auto")}
          ><IconArrowUp aria-hidden="true" /></Button
        >
      {/if}
    </div>
  </div>
  <!-- Session options sit under the card as icon controls: the same quiet
       row at every width, scrolling instead of wrapping when a narrow screen
       or many agent options leave no room. -->
  <div
    class="-my-1 flex min-w-0 flex-nowrap items-center gap-0.5 overflow-x-auto px-1 py-1 [scrollbar-width:none] max-sm:hidden"
    role="toolbar"
    aria-label="会话选项"
  >
    <ChatPermissionMode
      mode={chat.permissionMode}
      disabled={!!unavailable || savingMode}
      onchange={(mode) => void setPermissionMode(mode)}
    />
    <ChatConfig config={chat.config} {disabled} onchange={setConfig} />
  </div>
</div>
