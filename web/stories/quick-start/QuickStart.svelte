<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import IconAlertCircle from "~icons/tabler/alert-circle";
  import IconMenu2 from "~icons/tabler/menu-2";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconBook2 from "~icons/tabler/book-2";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconLoader2 from "~icons/tabler/loader-2";
  import IconNotes from "~icons/tabler/notes";
  import { Button } from "$lib/components/ui/button";
  import * as Popover from "$lib/components/ui/popover";
  import { Textarea } from "$lib/components/ui/textarea";
  import ChatConfig from "../../src/components/chat/ChatConfig.svelte";
  import ChatPermissionMode from "../../src/components/chat/ChatPermissionMode.svelte";
  import ChatSessionOptions from "../../src/components/chat/ChatSessionOptions.svelte";
  import LaunchConfigIcon from "../../src/components/LaunchConfigIcon.svelte";
  import type {
    ChatConfig as ChatOption,
    ChatPermissionMode as PermissionMode,
  } from "../../src/schemas";
  import { configs } from "../acp/fixtures";
  import AppShell from "./AppShell.svelte";
  import { liveEntry, type Mode } from "./data";
  import SessionPane from "./SessionPane.svelte";
  import WorkbenchHome from "./WorkbenchHome.svelte";
  import ValueSelect from "./ValueSelect.svelte";

  // Prototype of the app shell's two home pages (openspec change
  // quick-start-chat). Chat mode: write the task, pick a repository, base
  // branch and agent under the input, and one send creates a worktree,
  // starts the agent's chat side, applies the options and sends the first
  // message. Workbench mode: the existing overview beside the repository
  // tree. Opening a session shows it in the main area. Sample data only;
  // nothing leaves the page.
  type Scenario = "ready" | "no-options" | "sending" | "failed" | "missing";
  let {
    scenario = "ready",
    mode = "chat",
    session = null,
  }: { scenario?: Scenario; mode?: Mode; session?: string | null } = $props();

  const repositories = [
    {
      id: "webmux",
      name: "webmux",
      // Local branches only, as the new-worktree dialog offers.
      bases: ["main", "feat/quick-start-chat", "wt/brisk-otter"],
      instruction:
        "先阅读 AGENTS.md 和相关 docs/agent-rules，改完运行 make check；用 Conventional Commits 提交并开 draft PR。",
    },
    {
      id: "docs-site",
      name: "docs-site",
      bases: ["main"],
      instruction: "",
    },
    {
      id: "legacy-api",
      name: "legacy-api",
      bases: [],
      instruction: "",
      reason: "路径不可用：/srv/legacy-api 不存在",
    },
  ];
  const agents = [
    { id: "claudecode", name: "Claude Code", icon: "claudecode" },
    { id: "omp", name: "omp", icon: "pi" },
    { id: "opencode", name: "OpenCode", icon: "opencode" },
    { id: "codex", name: "Codex", icon: "codex" },
  ];
  // The option catalog each launch config last reported; Codex has never
  // started a chat side here, so it has none yet.
  const catalogs: Record<string, ChatOption[]> = {
    claudecode: configs.claudecode,
    omp: configs.omp,
    opencode: configs.opencode,
  };

  const initial = untrack(() => scenario);
  const available = initial === "missing" ? [] : agents;
  let repositoryId = $state("webmux");
  let base = $state("main");
  let agentId = $state(initial === "no-options" ? "codex" : "claudecode");
  let permissionMode = $state<PermissionMode>("auto");
  let text = $state(
    initial === "ready" || initial === "no-options"
      ? ""
      : "首页会话列表按最近活动排序，修复置顶会话被挤到后面的问题",
  );
  let sending = $state(initial === "sending");
  let failure = $state(initial === "failed");
  let notice = $state("");
  let options = $state.raw<ChatOption[]>(
    catalogs[untrack(() => agentId)] ?? [],
  );

  const repository = $derived(
    repositories.find((repo) => repo.id === repositoryId),
  );
  const agent = $derived(available.find((item) => item.id === agentId));
  const missing = $derived(
    !available.length
      ? "还没有支持聊天端的启动配置。"
      : !repositories.some((repo) => !repo.reason)
        ? "还没有可用的仓库。"
        : "",
  );
  const canSend = $derived(!missing && !sending && text.trim() !== "");

  let timer: ReturnType<typeof setTimeout> | undefined;
  onDestroy(() => clearTimeout(timer));

  function pickRepository(id: string) {
    repositoryId = id;
    base = repositories.find((repo) => repo.id === id)?.bases[0] ?? "";
  }
  function pickAgent(id: string) {
    agentId = id;
    options = catalogs[id] ?? [];
  }
  function setOption(id: string, value: { value?: string; enabled?: boolean }) {
    options = options.map((option) =>
      option.id === id ? { ...option, ...value } : option,
    );
  }
  function send() {
    if (!canSend) return;
    sending = true;
    failure = false;
    notice = "";
    timer = setTimeout(() => {
      sending = false;
      notice = `原型：已在 ${repository?.name} 基于 ${base} 新建 worktree，启动 ${agent?.name} 并发送首条消息，将在当前页进入聊天视图。`;
    }, 1500);
  }
  function keydown(event: KeyboardEvent) {
    if (event.isComposing || event.key !== "Enter" || event.shiftKey) return;
    event.preventDefault();
    send();
  }
</script>

<AppShell {mode} {session}>
  {#snippet children(view, openDrawer)}
    {@const open = view.session ? liveEntry(view.session) : undefined}
    {#if open}
      <SessionPane entry={open} />
    {:else if view.mode === "chat"}
      {@render page(openDrawer)}
    {:else}
      <WorkbenchHome scope={view.scope} worktree={view.worktree} {openDrawer} />
    {/if}
  {/snippet}
</AppShell>

{#snippet page(openDrawer: () => void)}
  <div class="flex h-full flex-col">
    <header class="flex h-12 shrink-0 items-center gap-1 border-b px-2 sm:px-4">
      <Button
        class="sm:hidden"
        variant="ghost"
        size="icon-touch"
        aria-label="打开导航"
        title="打开导航"
        onclick={openDrawer}><IconMenu2 aria-hidden="true" /></Button
      >
      <h1 class="text-sm font-medium">新聊天</h1>
    </header>

    <main
      class="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 px-4 text-center"
    >
      <h2 class="text-2xl font-medium">要让 agent 做什么？</h2>
      <p class="max-w-md text-sm text-muted-foreground">
        发送后会在所选仓库新建 worktree，以聊天方式启动 agent 并发出这条消息。
      </p>
      {#if notice}<p role="status" class="max-w-md text-sm text-success">
          {notice}
        </p>{/if}
    </main>

    <footer
      class="mx-auto w-full max-w-3xl space-y-1.5 px-3 pb-3 sm:px-4 sm:pb-6"
    >
      {#if missing}
        <div
          class="flex flex-wrap items-center gap-2 rounded-lg border px-3 py-2 text-sm"
          role="status"
        >
          <IconAlertCircle class="text-warning" aria-hidden="true" />
          <span class="mr-auto">{missing}</span>
          <Button variant="outline" size="sm-touch"
            >{available.length ? "添加仓库" : "新建启动配置"}</Button
          >
        </div>
      {/if}
      {#if failure}
        <div
          class="space-y-2 rounded-lg border border-destructive/40 px-3 py-2 text-sm"
          role="alert"
        >
          <p class="flex items-start gap-2">
            <IconAlertCircle
              class="mt-0.5 shrink-0 text-destructive"
              aria-hidden="true"
            />
            <span>
              <span class="font-medium">启动 agent 失败：</span>
              claude-agent-acp 在初始化阶段退出（exit 1）。消息未发送，已保留在输入框中。
            </span>
          </p>
          <p class="flex flex-wrap items-center gap-2 pl-6">
            <span
              class="min-w-0 truncate font-mono text-xs text-muted-foreground"
              title="/home/developer/webmux.worktrees/brisk-otter"
              >已创建 worktree：/home/developer/webmux.worktrees/brisk-otter</span
            >
            <Button variant="outline" size="sm-touch"
              >在此 worktree 新建会话</Button
            >
          </p>
        </div>
      {/if}

      <div
        class="rounded-lg border bg-card shadow-sm transition has-[textarea:focus]:border-ring has-[textarea:focus]:ring-3 has-[textarea:focus]:ring-ring/20"
      >
        {#if repository?.instruction}
          <div class="flex items-center border-b px-2 py-1">
            <Popover.Root>
              <Popover.Trigger>
                {#snippet child({ props })}
                  <Button
                    {...props}
                    variant="quiet"
                    size="xs"
                    title="查看将附带的仓库初始化指令"
                    ><IconNotes aria-hidden="true" />附带 {repository.name} 的初始化指令</Button
                  >
                {/snippet}
              </Popover.Trigger>
              <Popover.Content align="start" class="w-80 space-y-2 text-sm">
                <p class="font-medium">仓库初始化指令</p>
                <p class="whitespace-pre-wrap text-muted-foreground">
                  {repository.instruction}
                </p>
                <p class="text-xs text-muted-foreground">
                  会放在你的消息前面，作为第一条消息一起发送。在仓库菜单的“初始化指令”中修改。
                </p>
              </Popover.Content>
            </Popover.Root>
          </div>
        {/if}
        <div class="px-2 pt-1">
          <Textarea
            variant="bare"
            rows={2}
            class="max-h-48 min-h-14 resize-none"
            placeholder="描述任务，发送后在新 worktree 中开始"
            aria-label="任务描述"
            disabled={sending}
            bind:value={text}
            onkeydown={keydown}
          />
        </div>
        <div class="flex items-center justify-end gap-2 px-2 pb-2">
          {#if sending}
            <span
              class="mr-auto flex items-center gap-1.5 text-xs text-muted-foreground"
              role="status"
            >
              <IconLoader2
                class="size-3.5 animate-spin motion-reduce:animate-none"
                aria-hidden="true"
              />正在创建 worktree 并启动 {agent?.name}…
            </span>
          {:else if agent}
            <!-- Narrow screens keep the agent and its options in the card, as
               the chat composer does; the row below keeps the location. -->
            <span class="mr-auto flex min-w-0 items-center gap-0.5 sm:hidden">
              {@render agentPicker()}
              <ChatSessionOptions
                mode={permissionMode}
                config={options}
                disabled={sending}
                modeDisabled={sending}
                onmodechange={(mode) => (permissionMode = mode)}
                onconfigchange={setOption}
              />
            </span>
          {/if}
          <Button
            size="icon-touch"
            aria-label="发送"
            title="发送"
            disabled={!canSend}
            onclick={send}><IconArrowUp aria-hidden="true" /></Button
          >
        </div>
      </div>

      <div
        class="-my-1 flex min-w-0 flex-nowrap items-center gap-0.5 overflow-x-auto px-1 py-1 [scrollbar-width:none]"
        role="toolbar"
        aria-label="启动选项"
      >
        <ValueSelect
          name="仓库"
          value={repositoryId}
          choices={repositories.map((repo) => ({
            value: repo.id,
            name: repo.name,
            reason: repo.reason,
          }))}
          disabled={sending}
          onchange={pickRepository}
        >
          {#snippet icon()}<IconBook2 aria-hidden="true" />{/snippet}
        </ValueSelect>
        <ValueSelect
          name="基准分支"
          value={base}
          choices={(repository?.bases ?? []).map((name) => ({
            value: name,
            name,
          }))}
          disabled={sending}
          onchange={(value) => (base = value)}
        >
          {#snippet icon()}<IconGitBranch aria-hidden="true" />{/snippet}
        </ValueSelect>
        {#if agent}
          <div class="ml-auto flex shrink-0 items-center gap-0.5 max-sm:hidden">
            {@render agentPicker()}
            <ChatPermissionMode
              mode={permissionMode}
              disabled={sending}
              onchange={(mode) => (permissionMode = mode)}
            />
            {#if options.length}
              <ChatConfig
                config={options}
                disabled={sending}
                onchange={setOption}
              />
            {:else}
              <span
                class="shrink-0 px-2 text-xs text-muted-foreground"
                title="该 agent 还没有以聊天方式启动过；首次启动后会记住它的模型等配置"
                >配置首次启动后可选</span
              >
            {/if}
          </div>
        {/if}
      </div>
    </footer>
  </div>
{/snippet}

{#snippet agentPicker()}
  <ValueSelect
    name="Agent"
    value={agentId}
    choices={available.map((item) => ({ value: item.id, name: item.name }))}
    disabled={sending}
    onchange={pickAgent}
  >
    {#snippet icon(id)}<LaunchConfigIcon
        icon={available.find((item) => item.id === id)?.icon ?? ""}
        size="xs"
      />{/snippet}
  </ValueSelect>
{/snippet}
