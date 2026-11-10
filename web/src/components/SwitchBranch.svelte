<script lang="ts" module>
  import {
    previewSwitch,
    repositoryBranches,
    repositoryCommits,
    switchWorktree,
  } from "../api";
  import type {
    Branches,
    Commit,
    SwitchPreview,
    SwitchRequest,
    SwitchTarget,
    Worktree,
  } from "../schemas";

  /** Where the switch reads and writes; stories pass fixtures. */
  export interface SwitchSource {
    branches(id: string, signal: AbortSignal): Promise<Branches>;
    commits(
      id: string,
      from: readonly string[],
      signal: AbortSignal,
    ): Promise<{ commits: Commit[] }>;
    preview(
      tree: Worktree,
      target: SwitchTarget,
      signal: AbortSignal,
    ): Promise<SwitchPreview>;
    switch(tree: Worktree, request: SwitchRequest): Promise<Worktree>;
  }
  export const apiSwitchSource: SwitchSource = {
    branches: repositoryBranches,
    commits: repositoryCommits,
    preview: previewSwitch,
    switch: switchWorktree,
  };
</script>

<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import IconArrowRight from "~icons/tabler/arrow-right";
  import IconGitBranch from "~icons/tabler/git-branch";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import * as Select from "$lib/components/ui/select";
  import { APIError } from "../api";
  import {
    remoteBranchBases,
    remoteLocalName,
    suggestWorktreeBranch,
  } from "../branch-name";
  import { commitChoiceLabel, commitChoices } from "../commit-choices";
  import { errorMessage } from "../error";
  import { LatestRequest } from "../hooks/remote-request.svelte";
  import type { WorktreePreset } from "../new-session-defaults";
  import type { Repository } from "../schemas";
  import BlockerList from "./BlockerList.svelte";
  import BranchCommandList from "./BranchCommandList.svelte";
  import InlineNotice from "./InlineNotice.svelte";
  import PickerField from "./PickerField.svelte";
  import PreviewForm from "./PreviewForm.svelte";
  import WorktreeSessionList from "./WorktreeSessionList.svelte";

  // Switching a worktree's branch in place: choose an existing branch, a new
  // one or a commit to detach to; every choice is previewed by the server
  // (blockers, warnings, the sessions that keep running) and only a clean
  // preview can be confirmed. The same content sits in a dialog (home page,
  // sidebar) or in a repository panel subview; the host closes or goes back.
  let {
    repository,
    worktree,
    source = apiSwitchSource,
    busy = $bindable(false),
    ondone,
    onchange,
    onnewworktree,
    ongoto,
    oncancel,
  }: {
    repository: Repository;
    worktree: Worktree;
    source?: SwitchSource;
    /** The switch runs; the host blocks closing and going back. */
    busy?: boolean;
    /** The switch finished; the worktree as it is now. */
    ondone: (tree: Worktree) => void;
    /** A switch ran, whatever its outcome; the repositories are read again. */
    onchange: () => void;
    /** Opens the chosen target in a new worktree instead. */
    onnewworktree: (preset: WorktreePreset) => void;
    /** Shows the worktree at a path: the one that has the target checked out. */
    ongoto: (path: string) => void;
    oncancel: () => void;
  } = $props();

  type Mode = "existing" | "new" | "detach";
  let mode = $state<Mode>("existing");

  let data = $state<Branches | null>(null);
  let loadError = $state("");
  const branchRequests = new LatestRequest();
  async function readBranches() {
    const request = branchRequests.begin();
    loadError = "";
    try {
      const result = await source.branches(repository.id, request.signal);
      if (!request.current()) return;
      data = result;
      if (mode === "new" && !newName) newName = suggestion();
    } catch (err) {
      if (request.reportable()) loadError = errorMessage(err);
    }
  }

  const localNames = $derived(
    new Set((data?.branches ?? []).map((item) => item.name)),
  );
  const remotes = $derived(
    data ? remoteBranchBases(data.bases, localNames) : [],
  );
  // The current branch and branches checked out elsewhere stay listed,
  // disabled; so do remote branches whose local name already exists.
  const localItems = $derived(
    (data?.branches ?? []).map((item) => {
      const elsewhere = item.worktreePaths.find((p) => p !== worktree.path);
      return {
        name: item.name,
        reason:
          item.name === worktree.branch
            ? "当前分支"
            : elsewhere
              ? `已在 ${elsewhere} 检出`
              : undefined,
      };
    }),
  );
  const remoteItems = $derived(
    remotes.map((item) => {
      const local = remoteLocalName(item.name);
      return {
        name: item.name,
        reason: localNames.has(local)
          ? `本地已有 ${local}，请直接切换到该分支`
          : undefined,
      };
    }),
  );

  // Existing branch mode.
  let chosen = $state<{ kind: "local" | "remote"; name: string } | null>(null);
  let branchOpen = $state(false);
  let branchSearch = $state("");
  function setBranchOpen(next: boolean) {
    branchOpen = next;
    if (!next) branchSearch = "";
  }

  // New branch mode: a suggested name, started from the current branch (or
  // the current commit when detached).
  let newName = $state("");
  let startPoint = $state(
    untrack(() => worktree.branch ?? worktree.head ?? ""),
  );
  let startOpen = $state(false);
  let startSearch = $state("");
  function setStartOpen(next: boolean) {
    startOpen = next;
    if (!next) startSearch = "";
  }
  function suggestion(avoid?: string) {
    return suggestWorktreeBranch({
      branchNames: [...localNames],
      worktreePaths: repository.worktrees.map((tree) => tree.path),
      avoid,
    });
  }
  const nameTaken = $derived(localNames.has(newName.trim()));
  const startIsBranch = $derived(
    localNames.has(startPoint) || remotes.some((r) => r.name === startPoint),
  );

  // Detach mode: the recent history of the current commit.
  let commits = $state<Commit[]>([]);
  let commitsRead = $state(false);
  let commitsError = $state("");
  let commit = $state("");
  const commitRequests = new LatestRequest();
  async function readCommits() {
    const request = commitRequests.begin();
    commitsError = "";
    try {
      const result = worktree.head
        ? await source.commits(repository.id, [worktree.head], request.signal)
        : { commits: [] };
      if (request.current()) commits = result.commits;
    } catch (err) {
      if (request.reportable()) commitsError = errorMessage(err);
    } finally {
      if (request.current()) commitsRead = true;
    }
  }
  const commitRows = $derived(
    commitChoices(commits, {
      tips: worktree.head ? [{ hash: worktree.head, tag: "当前" }] : [],
      base: "",
      baseSubject: "",
    }),
  );
  const chosenCommit = $derived(commitRows.find((row) => row.value === commit));

  function changeMode(next: string) {
    if (next !== "existing" && next !== "new" && next !== "detach") return;
    if (next === mode) return;
    mode = next;
    setBranchOpen(false);
    setStartOpen(false);
    if (next === "new" && !newName && data) newName = suggestion();
    if (next === "detach" && !commitsRead) void readCommits();
  }

  const target = $derived.by((): SwitchTarget | null => {
    if (mode === "existing") {
      if (!chosen) return null;
      return chosen.kind === "local"
        ? { kind: "local", branch: chosen.name }
        : { kind: "remote", ref: chosen.name };
    }
    if (mode === "new") {
      const name = newName.trim();
      return name && !nameTaken && startPoint
        ? { kind: "new", name, startPoint }
        : null;
    }
    return commit ? { kind: "detach", commit } : null;
  });
  const targetKey = $derived(target ? JSON.stringify(target) : "");

  // Every choice is previewed again; a later choice drops the replies of
  // earlier ones. Typing a new name waits for a pause.
  let preview = $state<SwitchPreview | null>(null);
  let previewing = $state(false);
  let previewError = $state("");
  let acknowledged = $state(false);
  // Why the last confirmation was refused; the preview was read again.
  let conflict = $state("");
  let error = $state("");
  const previews = new LatestRequest();
  onDestroy(() => {
    previews.dispose();
    branchRequests.dispose();
    commitRequests.dispose();
  });
  void readBranches();
  async function readPreview(next: SwitchTarget) {
    const request = previews.begin();
    previewing = true;
    previewError = "";
    try {
      const result = await source.preview(worktree, next, request.signal);
      if (!request.current()) return;
      preview = result;
      acknowledged = false;
    } catch (err) {
      if (request.reportable()) {
        preview = null;
        previewError = errorMessage(err);
      }
    } finally {
      if (request.current()) previewing = false;
    }
  }
  $effect(() => {
    if (!targetKey) {
      untrack(() => {
        previews.cancel();
        preview = null;
        previewing = false;
        previewError = "";
      });
      return;
    }
    const next = untrack(() => target);
    if (!next) return;
    untrack(() => {
      // A reply for the previous choice must not land while this one waits.
      previews.cancel();
      preview = null;
      conflict = "";
      error = "";
      previewing = true;
    });
    const timer = setTimeout(
      () => void readPreview(next),
      next.kind === "new" ? 300 : 0,
    );
    return () => clearTimeout(timer);
  });

  function blocked(code: string) {
    return (preview?.blockers ?? []).some((item) => item.code === code);
  }
  const occupant = $derived(
    preview?.blockers.find((item) => item.code === "occupied" && item.path)
      ?.path,
  );
  const canSwitch = $derived(
    !!preview &&
      !previewing &&
      !busy &&
      preview.confirmationId !== "" &&
      !preview.blockers.length &&
      (!preview.sessions.length || acknowledged),
  );
  // Uncommitted changes, or sessions the user would rather not disturb,
  // leave a new worktree for the same target as the safer way.
  const newWorktreePreset = $derived.by((): WorktreePreset | null => {
    if (!preview || !target || target.kind === "detach") return null;
    if (!blocked("changes") && !preview.sessions.length) return null;
    if (blocked("occupied") || blocked("exists") || blocked("same"))
      return null;
    if (target.kind === "local")
      return { mode: "existing", branch: target.branch };
    if (target.kind === "remote")
      return {
        mode: "new",
        startBranch: target.ref,
        name: remoteLocalName(target.ref),
      };
    return {
      mode: "new",
      name: target.name,
      ...(startIsBranch ? { startBranch: target.startPoint } : {}),
    };
  });
  const sessionsTitle = $derived(
    preview?.warnings.find((item) => item.code === "sessions")?.message ??
      "运行中的会话",
  );
  const otherWarnings = $derived(
    (preview?.warnings ?? []).filter((item) => item.code !== "sessions"),
  );
  const agentWorking = $derived(
    (preview?.sessions ?? []).some(
      (item) => item.agent !== null && item.status === "running",
    ),
  );

  const short = (hash: string | null | undefined) => hash?.slice(0, 8) ?? "";
  const currentLabel = $derived.by(() => {
    const tree = preview?.worktree ?? worktree;
    return tree.branch ?? `分离 · ${short(tree.head) || "无提交"}`;
  });

  async function submit() {
    if (!canSwitch || !preview || !target) return;
    const confirmed = preview;
    const again = target;
    previews.cancel();
    busy = true;
    error = "";
    conflict = "";
    try {
      const tree = await source.switch(worktree, {
        confirmationId: confirmed.confirmationId,
        acknowledgeSessions: acknowledged,
      });
      onchange();
      ondone(tree);
      return;
    } catch (err) {
      if (err instanceof APIError && err.status === 409)
        conflict = errorMessage(err);
      else error = errorMessage(err);
      onchange();
    } finally {
      busy = false;
    }
    // The confirmation is spent either way; read the state as it is now.
    preview = null;
    acknowledged = false;
    void readPreview(again);
  }
</script>

{#snippet pickerFace(value: string, placeholder: string)}
  {#if value}
    <IconGitBranch
      class="size-4 shrink-0 text-muted-foreground"
      aria-hidden="true"
    />
    <span class="min-w-0 flex-1 truncate" title={value}>{value}</span>
  {:else}
    <span class="text-sm text-muted-foreground">
      {data || loadError ? placeholder : "正在读取分支…"}
    </span>
  {/if}
{/snippet}

<PreviewForm {busy} {error} onsubmit={() => void submit()} onclose={oncancel}>
  {#snippet summary()}
    <p
      class="rounded-lg border bg-muted/40 p-3 font-mono text-xs break-all text-muted-foreground"
    >
      {worktree.path}
    </p>
  {/snippet}
  <SegmentedControl.Root
    size="form"
    wrap
    value={mode}
    onValueChange={changeMode}
    disabled={busy}
    aria-label="切换目标"
  >
    <SegmentedControl.Item value="existing">已有分支</SegmentedControl.Item>
    <SegmentedControl.Item value="new">新建分支</SegmentedControl.Item>
    <SegmentedControl.Item value="detach">分离到提交</SegmentedControl.Item>
  </SegmentedControl.Root>
  {#if loadError}<p role="alert" class="text-sm break-all text-destructive">
      无法读取分支：{loadError}
    </p>{/if}

  {#if mode === "existing"}
    <div class="space-y-2">
      <Label for="switch-branch">目标分支</Label>
      <PickerField
        id="switch-branch"
        bind:open={() => branchOpen, setBranchOpen}
        disabled={busy || !data}
      >
        {#snippet trigger()}{@render pickerFace(
            chosen?.name ?? "",
            "选择本地或远程分支",
          )}{/snippet}
        <BranchCommandList
          label="目标分支"
          branches={localItems}
          remoteBranches={remoteItems}
          selected={chosen?.name ?? ""}
          loading={!data && !loadError}
          failed={!!loadError}
          disabled={busy}
          bind:search={branchSearch}
          onselect={(name) => {
            chosen = {
              kind: localNames.has(name) ? "local" : "remote",
              name,
            };
            setBranchOpen(false);
          }}
        />
      </PickerField>
      <p class="text-xs text-muted-foreground">
        远程分支会新建同名本地分支并设为其上游。
      </p>
    </div>
  {:else if mode === "new"}
    <div class="space-y-3">
      <div class="space-y-2">
        <div class="flex items-center justify-between gap-2">
          <Label for="switch-new-branch">新分支名称</Label>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={busy || !data}
            onclick={() => (newName = suggestion(newName || undefined))}
            >换一个</Button
          >
        </div>
        <Input
          id="switch-new-branch"
          bind:value={newName}
          disabled={busy}
          placeholder="feature/my-task"
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
        />
        {#if nameTaken}<p role="alert" class="text-xs text-destructive">
            已有同名本地分支；请换一个名称，或在“已有分支”中选择它。
          </p>{/if}
      </div>
      <div class="space-y-2">
        <Label for="switch-start-point">起点</Label>
        <PickerField
          id="switch-start-point"
          bind:open={() => startOpen, setStartOpen}
          disabled={busy || !data}
        >
          {#snippet trigger()}{@render pickerFace(
              startIsBranch || !startPoint
                ? startPoint
                : `当前提交 ${short(startPoint)}`,
              "选择起点",
            )}{/snippet}
          <BranchCommandList
            label="起点"
            branches={(data?.branches ?? []).map((item) => ({
              name: item.name,
            }))}
            remoteBranches={remotes.map((item) => ({ name: item.name }))}
            selected={startPoint}
            loading={!data && !loadError}
            failed={!!loadError}
            disabled={busy}
            bind:search={startSearch}
            onselect={(name) => {
              startPoint = name;
              setStartOpen(false);
            }}
          />
        </PickerField>
        <p class="text-xs text-muted-foreground">新分支不设置上游。</p>
      </div>
    </div>
  {:else}
    <div class="space-y-2">
      <Label for="switch-commit">提交</Label>
      <Select.Root
        type="single"
        bind:value={commit}
        disabled={busy || !commitRows.length}
      >
        <Select.Trigger id="switch-commit" class="w-full">
          {#if chosenCommit}
            <span class="min-w-0 flex-1 truncate" title={chosenCommit.subject}
              >{chosenCommit.subject} · {chosenCommit.shortHash}</span
            >
          {:else}
            <span class="text-muted-foreground">
              {commitsRead ? "选择提交" : "正在读取最近提交…"}
            </span>
          {/if}
        </Select.Trigger>
        <Select.Content
          class="max-h-[min(24rem,var(--bits-select-content-available-height))]"
        >
          {#each commitRows as row (row.value)}
            <Select.Item
              value={row.value}
              label={row.subject}
              aria-label={commitChoiceLabel(row)}
              >{commitChoiceLabel(row)}</Select.Item
            >
          {/each}
        </Select.Content>
      </Select.Root>
      {#if commitsError}<p role="alert" class="text-xs text-destructive">
          无法读取最近提交：{commitsError}
        </p>{:else if commitsRead && !commitRows.length}<p
          class="text-xs text-muted-foreground"
        >
          该工作树还没有提交。
        </p>{/if}
      <p class="text-xs text-muted-foreground">
        分离后工作树不在任何分支上，新提交需要另建分支保留。
      </p>
    </div>
  {/if}

  {#if previewing}
    <p role="status" class="text-sm text-muted-foreground">正在预览切换…</p>
  {:else if previewError}
    <p role="alert" class="text-sm break-all text-destructive">
      无法预览：{previewError}
    </p>
  {/if}
  {#if conflict}<InlineNotice tone="warning"
      >{conflict}；已按当前状态重新预览。</InlineNotice
    >{/if}
  {#if preview}
    <section aria-label="切换预览" class="space-y-3">
      <div class="space-y-1 rounded-lg border p-3 text-sm">
        <p
          class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 font-medium"
        >
          <span class="break-all">{currentLabel}</span>
          <IconArrowRight
            class="size-4 shrink-0 text-muted-foreground"
            aria-hidden="true"
          /><span class="sr-only">切换到</span>
          <span class="break-all"
            >{preview.target.branch ??
              `分离 · ${short(preview.target.commit)}`}</span
          >
        </p>
        <p class="text-xs break-all text-muted-foreground">
          <span class="font-mono">{short(preview.target.commit)}</span>
          · {preview.target.subject || "无标题提交"}
        </p>
        {#if preview.target.createsBranch}<p
            class="text-xs break-all text-muted-foreground"
          >
            将新建分支 {preview.target.branch}{preview.target.upstream
              ? `，上游为 ${preview.target.upstream}`
              : "，不设置上游"}。
          </p>{/if}
      </div>
      {#if preview.blockers.length}
        <BlockerList
          title="无法切换"
          blockers={preview.blockers.map((item) => item.message)}
        />
      {/if}
      {#if otherWarnings.length}
        <InlineNotice tone="warning">
          <ul class="space-y-1">
            {#each otherWarnings as item (item.code)}<li class="break-all">
                {item.message}
              </li>{/each}
          </ul>
        </InlineNotice>
      {/if}
      {#if preview.sessions.length}
        <WorktreeSessionList
          title={sessionsTitle}
          sessions={preview.sessions}
        />
        {#if agentWorking}<InlineNotice tone="warning"
            >有 agent 正在工作，可能正在写文件；切换后它的写入会落在新的分支上。</InlineNotice
          >{/if}
        {#if !preview.blockers.length}
          <Label class="items-start">
            <Checkbox
              bind:checked={acknowledged}
              disabled={busy}
              class="mt-0.5"
            />
            <span class="text-sm leading-snug"
              >我已了解：这些会话不会被关闭或重启，会直接看到切换后的文件。</span
            >
          </Label>
        {/if}
      {/if}
      {#if !preview.blockers.length}<p class="text-xs text-muted-foreground">
          不会贮藏或丢弃任何修改；确认 5 分钟内有效，状态变化后需要重新预览。
        </p>{/if}
    </section>
  {/if}

  {#snippet actions()}
    {#if occupant}
      {@const path = occupant}
      <Button
        type="button"
        variant="outline"
        wrap
        disabled={busy}
        title={path}
        onclick={() => ongoto(path)}>转到该工作树</Button
      >
    {/if}
    <Button type="submit" disabled={!canSwitch}
      >{busy ? "正在切换…" : "切换"}</Button
    >
    {#if newWorktreePreset}
      {@const preset = newWorktreePreset}
      <Button
        type="button"
        variant="success"
        wrap
        disabled={busy}
        title="不改动当前工作树与其中的会话，在新工作树中打开所选分支"
        onclick={() => onnewworktree(preset)}>改为在新工作树打开</Button
      >
    {/if}
  {/snippet}
</PreviewForm>
