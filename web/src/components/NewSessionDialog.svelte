<script lang="ts">
  import { errorMessage } from "../error";
  import { untrack } from "svelte";
  import {
    api,
    APIError,
    navigate,
    parseArgs,
    recentDirs,
    store,
    stored,
  } from "../api";
  import type { LaunchConfigsState } from "../launchConfigs";
  import {
    resolveInitialLaunch,
    resolveInitialTarget,
    selectableWorktree,
    worktreeLabel,
    type LaunchEntry,
    type LaunchSource,
  } from "../newSessionDefaults";
  import { hybridConflict } from "../recoverable";
  import ConnectorInstallHint from "./ConnectorInstallHint.svelte";
  import {
    CreateSessionRequestSchema,
    CreateWorktreeRequestSchema,
    NoContentSchema,
    SessionInfoSchema,
    WorktreeSchema,
    type AgentConnector,
    type Repository,
    type SessionInfo,
    type Worktree,
  } from "../schemas";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Tabs from "$lib/components/ui/tabs";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import AdvancedSection from "./AdvancedSection.svelte";
  import LaunchPicker from "./LaunchPicker.svelte";
  import LocationPicker from "./LocationPicker.svelte";
  import ResponsiveDialog from "./ResponsiveDialog.svelte";
  import WorktreeCreateFields from "./WorktreeCreateFields.svelte";
  import type { SplitDirection } from "../terminal-layout";

  let {
    launchConfigs,
    defaultCwd,
    repositories = [],
    entry,
    selectedRepositoryId = "",
    intent = "session",
    split = false,
    oncreated,
    onchange,
    onworktreecreated,
    onclose,
  }: {
    launchConfigs: LaunchConfigsState;
    defaultCwd: string;
    repositories?: Repository[];
    /** The contextual entry the dialog was opened from, if any. */
    entry?: LaunchEntry;
    /** The home page's selected repository; read only. */
    selectedRepositoryId?: string;
    intent?: "session" | "worktree";
    split?: boolean;
    oncreated?: (session: SessionInfo, direction: SplitDirection) => void;
    onchange: () => void;
    onworktreecreated: (tree: Worktree) => void;
    onclose: () => void;
  } = $props();
  const recent = recentDirs();
  const initial = untrack(() =>
    resolveInitialTarget({
      entry,
      selectedRepositoryId,
      repositories,
      recent,
      defaultCwd,
    }),
  );
  const preferredRepositoryId = initial.repositoryId;
  // A split keeps the pane's workspace while its directory is unchanged.
  const entryDirectory = untrack(() =>
    entry?.kind === "directory" ? entry : null,
  );
  let kind = $state(initial.kind);
  let cwd = $state(initial.cwd);
  let repositoryId = $state(initial.repositoryId);
  let worktreeId = $state(initial.worktreeId);
  let withSession = $state(untrack(() => intent === "session"));
  let direction = $state<SplitDirection>("horizontal");
  const treeOnly = $derived(intent === "worktree" && !withSession);

  const lastLaunch = stored("lastLaunch");
  let source = $state<LaunchSource>(
    lastLaunch === "custom" ? "custom" : "unset",
  );
  let configId = $state("");
  // Defaults apply once, and never after the user picked a source.
  let launchResolved = lastLaunch === "custom";
  let launchTouched = false;
  let command = $state(stored("customCommand"));
  let startSide = $state<"tui" | "acp">(
    stored("hybridStartSide") === "acp" ? "acp" : "tui",
  );
  let name = $state("");
  let error = $state("");
  // Starting on the chat side failed: its connector is not installed.
  let missingConnector = $state.raw<AgentConnector>();
  let busy = $state(false);
  let mkdirOpen = $state(false);
  // Avoid initializing an unused portal on every open; keep it mounted after
  // first use so closing animations and focus restoration run.
  let mkdirMounted = $state(false);

  const configs = $derived(
    launchConfigs.status === "ready" ? launchConfigs.configs : [],
  );
  const chosenConfig = $derived(
    source === "config"
      ? configs.find((config) => config.id === configId)
      : undefined,
  );
  $effect(() => {
    if (!launchResolved && !launchTouched) {
      const resolved = resolveInitialLaunch(lastLaunch, launchConfigs);
      if (resolved) {
        launchResolved = true;
        source = resolved.source;
        configId = resolved.configId;
      }
    }
    // Never silently replace a config deleted while the dialog is open.
    if (
      launchConfigs.status !== "loading" &&
      source === "config" &&
      !chosenConfig
    )
      source = "unset";
  });
  const commandReady = $derived.by(() => {
    if (source === "config") return !!chosenConfig;
    if (source !== "custom") return false;
    try {
      const argv = parseArgs(command);
      return !!argv.length && !!argv[0];
    } catch {
      return false;
    }
  });

  let newPath = $state("");
  let branchMode = $state<"new" | "existing">("new");
  let branch = $state("");
  let base = $state("");
  let startBranch = $state("");
  let worktreeFields = $state<WorktreeCreateFields>();
  // The repository of the new-worktree fields. They stay mounted once shown,
  // so typed fields survive switching location away and back; only choosing
  // a new worktree in another repository resets them.
  let fieldsRepositoryId = $state(
    initial.kind === "new-worktree" ? initial.repositoryId : "",
  );
  // Location changes come from the picker's bindings; whichever of the two
  // lands last sees both values.
  function followNewWorktree() {
    if (kind === "new-worktree") fieldsRepositoryId = repositoryId;
  }
  const fieldsRepository = $derived(
    repositories.find((r) => r.id === fieldsRepositoryId),
  );
  let createdTree = $state<Worktree | null>(null);
  const worktreeReady = $derived(worktreeFields?.isReady() ?? false);
  const repository = $derived(repositories.find((r) => r.id === repositoryId));
  const worktree = $derived(
    createdTree?.id === worktreeId
      ? createdTree
      : repository?.worktrees.find((w) => w.id === worktreeId),
  );
  // Creating only a worktree needs a new-worktree location; any other
  // location would have nothing to create.
  const treeTargetMissing = $derived(treeOnly && kind !== "new-worktree");
  const targetReady = $derived(
    !treeTargetMissing &&
      (kind === "directory"
        ? !!cwd.trim()
        : kind === "worktree"
          ? !!worktree && selectableWorktree(worktree)
          : !!repository && worktreeReady),
  );
  const newWorktreeSummary = $derived(
    branchMode === "new"
      ? `${branch || "新分支"} ← ${startBranch || "…"}`
      : `已有分支 ${branch || "未选择"}`,
  );
  const locationText = $derived(
    kind === "directory"
      ? cwd.trim()
      : kind === "worktree"
        ? worktree
          ? `${repository?.name ?? ""} · ${worktreeLabel(worktree)}`
          : ""
        : `${repository?.name ?? ""} 的新工作树 ${branch}`,
  );
  const summary = $derived.by(() => {
    if (treeTargetMissing)
      return "请将位置选为新建工作树，或勾选创建后启动会话";
    if (!targetReady)
      return kind === "new-worktree" ? "请填写新工作树信息" : "请选择位置";
    if (treeOnly) return `将创建 ${locationText}`;
    if (!commandReady)
      return source === "custom" ? "请输入可执行程序及参数" : "请选择启动方式";
    const launch =
      source === "config" ? (chosenConfig?.name ?? "") : command.trim();
    return `将在 ${locationText} 运行 ${launch}`;
  });

  async function create(makeDirectory = false) {
    if (busy) return;
    const onlyTree = treeOnly;
    if (onlyTree && kind !== "new-worktree") {
      error = "请将位置选为新建工作树，或勾选创建后启动会话。";
      return;
    }
    if (!onlyTree && !commandReady) {
      error = "请选择有效启动配置，或输入可执行程序及参数。";
      return;
    }
    busy = true;
    error = "";
    missingConnector = undefined;
    try {
      // Paint the pending state before a fast local response can immediately
      // start loading and mounting the terminal in the same interaction.
      await new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      );
      const custom = source === "custom";
      const cmd = custom && !onlyTree ? parseArgs(command) : undefined;
      if (custom && !onlyTree && (!cmd?.length || !cmd[0]))
        throw new Error("请输入可执行程序及参数");
      if (kind === "new-worktree") {
        const tree = await api(
          `/repositories/${repositoryId}/worktrees`,
          WorktreeSchema,
          "POST",
          {
            path: newPath,
            mode: branchMode,
            branch,
            ...(branchMode === "new" ? { base } : {}),
          },
          undefined,
          CreateWorktreeRequestSchema,
        );
        createdTree = tree;
        worktreeId = tree.id;
        kind = "worktree";
        onworktreecreated(tree);
        onchange();
      }
      if (onlyTree) {
        onclose();
        return;
      }
      const workspace =
        entryDirectory?.workspace && cwd === entryDirectory.cwd
          ? entryDirectory.workspace
          : null;
      const target =
        kind === "directory"
          ? {
              cwd,
              ...(workspace
                ? {
                    workspace: {
                      repositoryId: workspace.repositoryId,
                      worktreeId: workspace.worktreeId,
                    },
                  }
                : {}),
            }
          : { workspace: { repositoryId, worktreeId } };
      const payload = chosenConfig
        ? {
            ...target,
            configId: chosenConfig.id,
            name,
            ...(chosenConfig.hybrid ? { startSide } : {}),
          }
        : { ...target, cmd, name };
      if (makeDirectory)
        await api("/fs/mkdir", NoContentSchema, "POST", { path: cwd });
      const session = await api(
        "/sessions",
        SessionInfoSchema,
        "POST",
        payload,
        undefined,
        CreateSessionRequestSchema,
      );
      store(
        "recentDirs",
        JSON.stringify(
          [session.cwd, ...recent.filter((p) => p !== session.cwd)].slice(
            0,
            10,
          ),
        ),
      );
      store(
        "lastLaunch",
        chosenConfig ? `config:${chosenConfig.id}` : "custom",
      );
      if (chosenConfig?.hybrid) store("hybridStartSide", startSide);
      if (custom) store("customCommand", command);
      if (oncreated) oncreated(session, direction);
      else navigate("/s/" + session.id);
    } catch (err) {
      if (kind !== "directory") onchange();
      const conflict = hybridConflict(err);
      if (
        kind === "directory" &&
        !makeDirectory &&
        err instanceof APIError &&
        err.status === 400 &&
        err.message.startsWith("directory:") &&
        err.message.includes("no such file or directory")
      ) {
        mkdirMounted = true;
        mkdirOpen = true;
      } else if (conflict?.reason === "connector_missing" && conflict.connector)
        missingConnector = conflict.connector;
      else error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

{#snippet fields()}
  <form
    class="min-w-0"
    onsubmit={(event) => {
      event.preventDefault();
      void create();
    }}
  >
    <fieldset disabled={busy} class="min-w-0 space-y-4">
      <LocationPicker
        {repositories}
        {recent}
        {preferredRepositoryId}
        {createdTree}
        {newWorktreeSummary}
        bind:kind={
          () => kind,
          (next) => {
            kind = next;
            followNewWorktree();
          }
        }
        bind:cwd
        bind:repositoryId={
          () => repositoryId,
          (next) => {
            repositoryId = next;
            followNewWorktree();
          }
        }
        bind:worktreeId
        disabled={busy}
      />
      {#if fieldsRepository}
        <fieldset
          disabled={busy || kind !== "new-worktree"}
          hidden={kind !== "new-worktree"}
          class="min-w-0"
        >
          {#key fieldsRepository.id}<WorktreeCreateFields
              bind:this={worktreeFields}
              repository={fieldsRepository}
              disabled={busy || kind !== "new-worktree"}
              bind:path={newPath}
              bind:mode={branchMode}
              bind:branch
              bind:base
              bind:startBranch
            />{/key}
        </fieldset>
      {/if}
      {#if createdTree && kind === "worktree" && worktreeId === createdTree.id}
        <p role="status" class="text-sm text-primary">
          工作树已创建。再次提交只启动会话，不会重复创建分支或目录。
        </p>
      {/if}
      {#if intent === "worktree"}
        <div class="flex items-center gap-2">
          <Checkbox id="worktree-with-session" bind:checked={withSession} />
          <Label for="worktree-with-session">创建后启动会话</Label>
        </div>
      {/if}
      {#if !treeOnly}
        <LaunchPicker
          {launchConfigs}
          bind:source
          bind:configId
          bind:command
          bind:startSide
          disabled={busy}
          onchoose={() => {
            launchTouched = true;
          }}
        />
        {#if split}
          <Tabs.Root bind:value={direction}>
            <Tabs.List aria-label="分屏方向" size="form" class="w-full">
              <Tabs.Trigger value="horizontal">左右分屏</Tabs.Trigger>
              <Tabs.Trigger value="vertical">上下分屏</Tabs.Trigger>
            </Tabs.List>
          </Tabs.Root>
        {/if}
        <AdvancedSection title="高级：会话名称" summary={name}>
          <div class="space-y-2">
            <Label for="session-name"
              >会话名称 <span class="text-muted-foreground">可选</span></Label
            ><Input
              id="session-name"
              bind:value={name}
              placeholder="留空则跟随终端标题"
            />
          </div>
        </AdvancedSection>
      {/if}
      {#if error}<p role="alert" class="text-sm text-destructive">
          {error}
        </p>{/if}
      {#if missingConnector}
        <ConnectorInstallHint connector={missingConnector}>
          {#snippet actions()}<Button
              type="button"
              size="sm"
              variant="outline"
              disabled={busy}
              onclick={() => {
                startSide = "tui";
                void create();
              }}>改为从终端开始</Button
            >{/snippet}
        </ConnectorInstallHint>
      {/if}
      <div
        class="flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center"
      >
        <p
          id="session-summary"
          class="min-w-0 flex-1 text-sm wrap-anywhere text-muted-foreground"
        >
          {summary}
        </p>
        <div class="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onclick={onclose}>取消</Button
          >
          <Button
            type="submit"
            aria-describedby="session-summary"
            disabled={busy || !targetReady || (!treeOnly && !commandReady)}
            >{busy
              ? treeOnly
                ? "正在创建…"
                : "正在启动…"
              : treeOnly
                ? "创建工作树"
                : split
                  ? "创建并加入分屏 →"
                  : "创建并进入 →"}</Button
          >
        </div>
      </div>
    </fieldset>
  </form>
{/snippet}

<ResponsiveDialog
  title={treeOnly ? "新建工作树" : "新建会话"}
  description={treeOnly
    ? "创建独立的代码副本，可选择同时启动会话。"
    : "浏览器断开后，会话仍会继续。"}
  class="p-5 sm:max-w-3xl"
  pinDrawerHeader
  {busy}
  {onclose}
>
  {@render fields()}
</ResponsiveDialog>

{#if mkdirMounted}<AlertDialog.Root bind:open={mkdirOpen}>
    <AlertDialog.Content>
      <AlertDialog.Header
        ><AlertDialog.Title>目录不存在，是否创建？</AlertDialog.Title
        ><AlertDialog.Description class="break-all"
          >将在 {cwd} 创建目录，然后启动会话。</AlertDialog.Description
        ></AlertDialog.Header
      >
      <AlertDialog.Footer>
        <AlertDialog.Cancel>取消</AlertDialog.Cancel>
        <AlertDialog.Action
          onclick={() => {
            void create(true);
          }}>创建目录并启动</AlertDialog.Action
        >
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>{/if}
