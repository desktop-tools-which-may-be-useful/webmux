<script lang="ts">
  import { onMount, untrack } from "svelte";
  import IconAlertCircle from "~icons/tabler/alert-circle";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconBook2 from "~icons/tabler/book-2";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconLoader2 from "~icons/tabler/loader-2";
  import IconNotes from "~icons/tabler/notes";
  import { Button } from "$lib/components/ui/button";
  import * as Popover from "$lib/components/ui/popover";
  import { Textarea } from "$lib/components/ui/textarea";
  import { api } from "../api";
  import { suggestWorktreeBranch } from "../branchName";
  import { setBrowserChrome, resetBrowserChrome } from "../browser-chrome";
  import type { ConfigValue } from "../chat-api";
  import ChatConfig from "../components/chat/ChatConfig.svelte";
  import ChatPermissionMode from "../components/chat/ChatPermissionMode.svelte";
  import ChatSelect from "../components/chat/ChatSelect.svelte";
  import ChatSessionOptions from "../components/chat/ChatSessionOptions.svelte";
  import ConnectorInstallHint from "../components/ConnectorInstallHint.svelte";
  import LaunchConfigIcon from "../components/LaunchConfigIcon.svelte";
  import SidebarButton from "../components/SidebarButton.svelte";
  import { errorMessage } from "../error";
  import {
    configValues,
    failedStep,
    pickDefault,
    quickStartMemory,
    sendQuickStart,
  } from "../quickStart";
  import {
    BranchesSchema,
    type ChatConfig as ChatOption,
    type ChatPermissionMode as PermissionMode,
    type QuickStartFailure,
    type Repository,
  } from "../schemas";
  import { getShell, openSession } from "../shell.svelte";
  import { z } from "zod";

  // Chat mode's home (openspec quick-start-chat): write the task, pick the
  // repository, base branch and agent under the input, and one send makes
  // the server create a worktree, start the agent's chat side, apply the
  // options and send the first message. Laid out like the Claude Code web
  // composer: an empty stage with the input card at the bottom.
  const shell = getShell();
  type Branches = z.infer<typeof BranchesSchema>;

  const repositories = $derived(shell.repositories.list);
  const agents = $derived(
    shell.launchConfigs.state.status === "ready"
      ? shell.launchConfigs.state.configs.filter((config) => config.hybrid)
      : [],
  );
  const remembered = quickStartMemory.choice();
  let repositoryChoice = $state(remembered?.repositoryId);
  let agentChoice = $state(remembered?.configId);
  const usableRepository = (repo: Repository) =>
    repo.availability === "available";
  const repository = $derived(
    pickDefault(repositories, repositoryChoice, usableRepository),
  );
  const agent = $derived(pickDefault(agents, agentChoice, () => true));

  let text = $state(quickStartMemory.draft());
  $effect(() => quickStartMemory.saveDraft(text));
  let permissionMode = $state<PermissionMode>("ask");
  // The page's option values: the agent's remembered catalog plus edits.
  let edits = $state<Record<string, Record<string, ConfigValue>>>({});
  const options = $derived<ChatOption[]>(
    (agent?.chatOptions ?? []).map((option) => {
      const edit = agent ? edits[agent.id]?.[option.id] : undefined;
      return edit ? { ...option, ...edit } : option;
    }),
  );
  function setOption(id: string, value: ConfigValue) {
    if (!agent) return;
    edits = { ...edits, [agent.id]: { ...edits[agent.id], [id]: value } };
  }

  // The base branch defaults to the repository's main worktree branch and
  // offers its local branches, as the new-worktree dialog does.
  let branches = $state<{ id: string; data?: Branches; error?: string }>();
  let baseChoice = $state<{ repositoryId: string; head: string }>();
  // Keyed on the ID alone: a refreshed repository list must not cancel the
  // read in flight.
  const repositoryId = $derived(repository?.id);
  $effect(() => {
    const id = repositoryId;
    if (!id || untrack(() => branches?.id) === id) return;
    branches = { id };
    const controller = new AbortController();
    api(
      `/repositories/${id}/branches`,
      BranchesSchema,
      "GET",
      undefined,
      controller.signal,
    )
      .then((data) => {
        if (branches?.id === id) branches = { id, data };
      })
      .catch((err: unknown) => {
        if (branches?.id === id && !controller.signal.aborted)
          branches = { id, error: errorMessage(err) };
      });
    return () => controller.abort();
  });
  const baseChoices = $derived.by(() => {
    const data = branches?.id === repository?.id ? branches?.data : undefined;
    if (!data) return [];
    const local = data.branches.map((branch) => ({
      value: branch.head,
      name: branch.name,
    }));
    if (data.defaultBase && !local.some((b) => b.value === data.defaultBase))
      local.unshift({
        value: data.defaultBase,
        name: data.defaultBranch ?? "HEAD",
      });
    return local;
  });
  const base = $derived(
    baseChoice?.repositoryId === repository?.id &&
      baseChoices.some((choice) => choice.value === baseChoice?.head)
      ? (baseChoice?.head ?? "")
      : (branches?.data?.defaultBase ?? ""),
  );
  const baseName = $derived(
    baseChoices.find((choice) => choice.value === base)?.name ?? "",
  );

  const missing = $derived(
    shell.launchConfigs.state.status === "ready" && !agents.length
      ? "agent"
      : shell.repositories.settled && !repositories.some(usableRepository)
        ? "repository"
        : "",
  );
  let sending = $state(false);
  let failure = $state<QuickStartFailure>();
  const canSend = $derived(
    !sending && !!repository && !!agent && !!base && text.trim() !== "",
  );

  let input = $state<HTMLTextAreaElement | null>(null);
  onMount(() => {
    setBrowserChrome("新聊天 · WebMux", false);
    input?.focus();
    return resetBrowserChrome;
  });

  async function send() {
    if (!canSend || !repository || !agent || !branches?.data) return;
    sending = true;
    failure = undefined;
    const data = branches.data;
    const taken = data.branches.map((branch) => branch.name);
    const paths = repository.worktrees.map((tree) => tree.path);
    let avoid: string | undefined;
    try {
      // A branch or directory that appeared since the list was read is a
      // conflict; draw another name and send again under a new request ID.
      for (let attempt = 0; attempt < 3; attempt++) {
        const branch = suggestWorktreeBranch({
          branchNames: taken,
          worktreePaths: paths,
          avoid,
        });
        const outcome = await sendQuickStart({
          repositoryId: repository.id,
          configId: agent.id,
          base,
          branch,
          text,
          config: configValues(options),
          permissionMode,
        });
        if (outcome.ok) {
          quickStartMemory.rememberChoice({
            repositoryId: repository.id,
            configId: agent.id,
          });
          text = "";
          quickStartMemory.clearDraft();
          openSession({ id: outcome.sessionId });
          return;
        }
        if (
          outcome.failure.step !== "worktree" ||
          outcome.failure.reason !== "conflict"
        ) {
          failure = outcome.failure;
          return;
        }
        taken.push(branch);
        avoid = branch;
        failure = outcome.failure;
      }
    } finally {
      sending = false;
      void shell.repositories.refresh();
    }
  }
  function keydown(event: KeyboardEvent) {
    if (event.isComposing || event.key !== "Enter" || event.shiftKey) return;
    event.preventDefault();
    void send();
  }
</script>

<main class="flex h-dvh flex-col">
  <header
    class="app-header flex min-h-(--app-header-height) shrink-0 items-center gap-1 border-b bg-card px-1 sm:px-4"
  >
    <SidebarButton class="sm:hidden" />
    <h1 class="text-sm font-medium">新聊天</h1>
  </header>

  <div
    class="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 px-4 text-center"
  >
    <h2 class="text-2xl font-medium">要让 agent 做什么？</h2>
    <p class="max-w-md text-sm text-muted-foreground">
      发送后会在所选仓库新建 worktree，以聊天方式启动 agent 并发出这条消息。
    </p>
  </div>

  <footer
    class="mx-auto w-full max-w-3xl space-y-1.5 px-3 pb-safe-4 sm:px-4 sm:pb-6"
  >
    {#if missing}
      <div
        class="flex flex-wrap items-center gap-2 rounded-lg border px-3 py-2 text-sm"
        role="status"
      >
        <IconAlertCircle class="text-warning" aria-hidden="true" />
        <span class="mr-auto">
          {missing === "agent"
            ? "还没有勾选混合模式的启动配置，无法以聊天方式启动 agent。"
            : "还没有可用的仓库。"}
        </span>
        {#if missing === "agent"}
          <Button
            variant="outline"
            size="sm-touch"
            href="/settings#launch-configs">新建启动配置</Button
          >
        {:else}
          <Button
            variant="outline"
            size="sm-touch"
            onclick={shell.addRepository}>添加仓库</Button
          >
        {/if}
      </div>
    {/if}
    {#if branches?.error && branches.id === repository?.id}
      <p role="alert" class="px-1 text-sm text-destructive">
        读取 {repository?.name} 的分支失败：{branches.error}
      </p>
    {/if}
    {#if failure}
      {@const failed = failure}
      <div
        class="space-y-2 rounded-lg border border-destructive/40 px-3 py-2 text-sm"
        role="alert"
      >
        <p class="flex items-start gap-2">
          <IconAlertCircle
            class="mt-0.5 shrink-0 text-destructive"
            aria-hidden="true"
          />
          <span class="min-w-0 break-words">
            <span class="font-medium">{failedStep[failed.step]}：</span
            >{failed.error}
            {failed.sessionId ? "" : "消息未发送，已保留在输入框中。"}
          </span>
        </p>
        {#if failed.connector}
          <ConnectorInstallHint connector={failed.connector} />
        {/if}
        {#if failed.worktree || failed.sessionId}
          <p class="flex flex-wrap items-center gap-2 pl-6">
            {#if failed.worktree}
              {@const tree = failed.worktree}
              <span
                class="min-w-0 truncate font-mono text-xs text-muted-foreground"
                title={tree.path}>已创建 worktree：{tree.path}</span
              >
              {#if !failed.sessionId}
                <Button
                  variant="outline"
                  size="sm-touch"
                  onclick={() =>
                    shell.start({
                      kind: "worktree",
                      repositoryId: tree.repositoryId,
                      worktreeId: tree.id,
                    })}>在此 worktree 新建会话</Button
                >
              {/if}
            {/if}
            {#if failed.sessionId}
              {@const id = failed.sessionId}
              <Button
                variant="outline"
                size="sm-touch"
                onclick={() => openSession({ id })}>进入会话</Button
              >
            {/if}
          </p>
        {/if}
      </div>
    {/if}

    <div
      class="rounded-lg border bg-card shadow-sm transition has-[textarea:focus]:border-ring has-[textarea:focus]:ring-3 has-[textarea:focus]:ring-ring/20"
    >
      {#if repository?.initInstruction}
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
            <Popover.Content align="start" class="w-80 space-y-2">
              <p class="font-medium">仓库初始化指令</p>
              <p class="whitespace-pre-wrap break-words text-muted-foreground">
                {repository.initInstruction}
              </p>
              <p class="text-xs text-muted-foreground">
                会放在你的消息前面，空一行后作为第一条消息一起发送。在仓库菜单的“初始化指令…”中修改。
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
          bind:ref={input}
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
          onclick={() => void send()}><IconArrowUp aria-hidden="true" /></Button
        >
      </div>
    </div>

    <div
      class="-my-1 flex min-w-0 flex-nowrap items-center gap-0.5 overflow-x-auto px-1 py-1 [scrollbar-width:none]"
      role="toolbar"
      aria-label="启动选项"
    >
      <ChatSelect
        labeled
        name="仓库"
        value={repository?.id ?? ""}
        choices={repositories.map((repo) => ({
          value: repo.id,
          name: repo.name,
          reason: usableRepository(repo)
            ? undefined
            : repo.error || "仓库当前不可用",
        }))}
        disabled={sending}
        onchange={(id) => (repositoryChoice = id)}
      >
        {#snippet icon()}<IconBook2 aria-hidden="true" />{/snippet}
      </ChatSelect>
      <ChatSelect
        labeled
        name="基准分支"
        value={base}
        choices={baseChoices}
        disabled={sending || !baseChoices.length}
        title={baseName ? `基准分支：${baseName}` : "基准分支"}
        onchange={(head) =>
          (baseChoice = { repositoryId: repository?.id ?? "", head })}
      >
        {#snippet icon()}<IconGitBranch aria-hidden="true" />{/snippet}
      </ChatSelect>
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
</main>

{#snippet agentPicker()}
  <ChatSelect
    labeled
    name="Agent"
    value={agent?.id ?? ""}
    choices={agents.map((item) => ({ value: item.id, name: item.name }))}
    disabled={sending}
    onchange={(id) => (agentChoice = id)}
  >
    {#snippet choiceIcon(id)}<LaunchConfigIcon
        icon={agents.find((item) => item.id === id)?.icon ?? ""}
        size="xs"
      />{/snippet}
  </ChatSelect>
{/snippet}
