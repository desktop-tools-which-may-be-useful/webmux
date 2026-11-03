<script lang="ts">
  import { errorMessage } from "../error";
  import { api } from "../api";
  import { suggestWorktreeBranch } from "../branchName";
  import {
    BranchesSchema,
    RefreshBranchBaseRequestSchema,
    RefreshBranchBaseResponseSchema,
    type Repository,
  } from "../schemas";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import IconRefresh from "~icons/tabler/refresh";
  import * as Command from "$lib/components/ui/command";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import type { z } from "zod";
  let {
    repository,
    path = $bindable(""),
    mode = $bindable<"new" | "existing">("new"),
    branch = $bindable(""),
    base = $bindable(""),
    disabled = false,
  }: {
    repository: Repository;
    path: string;
    mode: "new" | "existing";
    branch: string;
    base: string;
    disabled?: boolean;
  } = $props();
  let data = $state<z.infer<typeof BranchesSchema> | null>(null);
  let error = $state("");
  let loading = $state(false);
  let startBranch = $state("");
  let refreshing = $state(false);
  let refreshWarning = $state("");
  let upstream = $state("");
  let baseSource = $state<"local" | "remote" | "">("");
  let baseSubject = $state("");
  let lastRefreshedAt = $state<Date | null>(null);
  let refreshGeneration = 0;
  let baseEditVersion = 0;
  let refreshController: AbortController | null = null;
  let pathEdited = $state(false);
  let branchEdited = $state(false);
  const baseChoices = $derived([
    ...(data?.defaultBase
      ? [
          {
            value: data.defaultBase,
            label: `${data.defaultBaseSubject || "无标题提交"}（当前主工作区）`,
          },
        ]
      : []),
    ...(data?.bases.map((item) => ({
      value: item.name,
      label: `${item.subject || "无标题提交"}（${item.name.replace(/^refs\/heads\//, "本地：").replace(/^refs\/remotes\//, "远程：")}）`,
    })) || []),
  ]);
  const baseLabel = $derived(
    baseChoices.find((item) => item.value === base)?.label ?? base,
  );

  function suggestedBranch(avoid?: string) {
    if (!data) return "";
    return suggestWorktreeBranch({
      branchNames: data.branches.map((item) => item.name),
      worktreePaths: repository.worktrees.map((tree) => tree.path),
      avoid,
    });
  }
  function fillSuggestedBranch() {
    if (mode !== "new" || branchEdited || branch || !data) return;
    branch = suggestedBranch();
  }

  function clearRefresh() {
    refreshGeneration += 1;
    refreshController?.abort();
    refreshController = null;
    refreshing = false;
    refreshWarning = "";
    upstream = "";
    lastRefreshedAt = null;
  }

  function refreshUpstream(branchName: string) {
    const generation = ++refreshGeneration;
    const editVersion = baseEditVersion;
    const repositoryID = repository.id;
    refreshController?.abort();
    const controller = new AbortController();
    refreshController = controller;
    refreshing = true;
    refreshWarning = "";
    void api(
      `/repositories/${repositoryID}/branches/refresh`,
      RefreshBranchBaseResponseSchema,
      "POST",
      { branch: branchName },
      controller.signal,
      RefreshBranchBaseRequestSchema,
    )
      .then((value) => {
        if (
          generation !== refreshGeneration ||
          mode !== "new" ||
          repository.id !== repositoryID ||
          startBranch !== branchName
        )
          return;
        upstream = value.upstream;
        refreshWarning = value.fetchWarning;
        lastRefreshedAt = value.baseSource === "remote" ? new Date() : null;
        if (editVersion === baseEditVersion) {
          base = value.base;
          baseSource = value.baseSource;
          baseSubject = value.baseSubject;
        }
      })
      .catch((err: unknown) => {
        if (generation !== refreshGeneration) return;
        if (err instanceof DOMException && err.name === "AbortError") return;
        refreshWarning = "上游刷新失败，保留当前基线；请重试";
        baseSource = "";
        upstream = "";
        lastRefreshedAt = null;
      })
      .finally(() => {
        if (generation !== refreshGeneration) return;
        refreshing = false;
        refreshController = null;
      });
  }

  function chooseStartBranch(name: string) {
    startBranch = name;
    clearRefresh();
    const selected = data?.branches.find((item) => item.name === name);
    if (!selected) {
      base = data?.defaultBase || "";
      baseSource = "";
      baseSubject = data?.defaultBaseSubject || "";
      return;
    }
    base = selected.head;
    baseSource = "local";
    baseSubject =
      data?.bases.find((item) => item.name === `refs/heads/${name}`)?.subject ||
      "";
    refreshUpstream(name);
  }

  $effect(() => {
    const id = repository.id;
    let active = true;
    clearRefresh();
    data = null;
    startBranch = "";
    baseSource = "";
    baseSubject = "";
    error = "";
    loading = true;
    pathEdited = false;
    branchEdited = false;
    branch = "";
    base = "";
    void api(`/repositories/${id}/branches`, BranchesSchema)
      .then((value) => {
        if (!active) return;
        data = value;
        base = value.defaultBase || "";
        baseSubject = value.defaultBaseSubject;
        fillSuggestedBranch();
        startBranch = value.defaultBranch || "";
        if (mode === "new" && startBranch) chooseStartBranch(startBranch);
      })
      .catch((err) => {
        if (active) error = errorMessage(err);
      })
      .finally(() => {
        if (active) loading = false;
      });
    return () => {
      active = false;
      clearRefresh();
    };
  });
  $effect(() => {
    if (!pathEdited)
      path = data
        ? `${data.defaultParent}/${branch.split("/").at(-1) || ""}`
        : "";
  });
  const ready = $derived(
    !!data && !loading && !!branch && !!path && (mode === "existing" || !!base),
  );
  export function isReady() {
    return ready;
  }
</script>

<div class="space-y-4 rounded-lg border p-3">
  <fieldset class="space-y-2">
    <legend class="mb-2 text-sm font-medium">工作树分支</legend>
    <div class="flex flex-wrap gap-2">
      <Button
        type="button"
        variant={mode === "new" ? "default" : "outline"}
        aria-pressed={mode === "new"}
        onclick={() => {
          if (mode === "new") return;
          mode = "new";
          branchEdited = false;
          branch = "";
          fillSuggestedBranch();
          if (startBranch) chooseStartBranch(startBranch);
        }}>创建新分支</Button
      >
      <Button
        type="button"
        variant={mode === "existing" ? "default" : "outline"}
        aria-pressed={mode === "existing"}
        onclick={() => {
          if (mode === "existing") return;
          mode = "existing";
          clearRefresh();
          baseSource = "";
          branch = "";
        }}>使用已有分支</Button
      >
    </div>
  </fieldset>
  {#if loading}<p role="status" class="text-sm">正在读取本地分支…</p>{/if}
  {#if error}<p role="alert" class="break-all text-sm text-destructive">
      {error}
    </p>{/if}
  {#if mode === "new"}
    <div class="space-y-2">
      <div class="flex items-center justify-between gap-2">
        <Label for="worktree-start-branch">起始分支</Label>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!startBranch || refreshing || disabled}
          aria-busy={refreshing}
          onclick={() => refreshUpstream(startBranch)}
          ><IconRefresh
            class={refreshing ? "animate-spin motion-reduce:animate-none" : ""}
            aria-hidden="true"
          />{refreshing ? "正在刷新" : "刷新上游"}</Button
        >
      </div>
      <NativeSelect.Root
        id="worktree-start-branch"
        class="w-full"
        value={startBranch}
        disabled={disabled || loading || !data}
        onchange={(event) => chooseStartBranch(event.currentTarget.value)}
      >
        {#if !startBranch}<NativeSelect.Option value=""
            >选择本地分支</NativeSelect.Option
          >{/if}
        {#each data?.branches || [] as item (item.name)}
          <NativeSelect.Option value={item.name}
            >{item.name}</NativeSelect.Option
          >
        {/each}
      </NativeSelect.Root>
      {#if refreshing}
        <p
          role="status"
          aria-label="上游刷新状态"
          class="text-xs text-muted-foreground"
        >
          正在获取 {startBranch} 对应的上游最新提交；可继续编辑或创建。
        </p>
      {:else if baseSource === "remote" && upstream}
        <p
          role="status"
          aria-label="上游刷新状态"
          class="break-all text-xs text-muted-foreground"
        >
          远程最新 · {upstream} · {baseSubject || "无标题提交"}
          {#if lastRefreshedAt}
            · 最近刷新 {lastRefreshedAt.toLocaleTimeString()}{/if}
        </p>
      {:else if baseSource === "local" && startBranch}
        <p
          role="status"
          aria-label="上游刷新状态"
          class="break-all text-xs text-muted-foreground"
        >
          本地：{startBranch}
        </p>
      {/if}
      {#if refreshWarning}
        <p role="status" aria-label="上游刷新警告" class="text-xs text-warning">
          {refreshWarning}
        </p>
      {/if}
    </div>
    <div class="space-y-2">
      <div class="flex items-center justify-between gap-2">
        <Label for="worktree-branch">新分支名称</Label>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!data || loading}
          onclick={() => {
            branch = suggestedBranch(branch || undefined);
          }}>换一个</Button
        >
      </div>
      <Input
        id="worktree-branch"
        bind:value={branch}
        oninput={() => {
          branchEdited = true;
        }}
        required
        placeholder="feature/my-task"
        autocomplete="off"
        autocapitalize="off"
        autocorrect="off"
        spellcheck={false}
      />
      <p class="text-xs text-muted-foreground">
        建议名避开已有本地分支和已有工作树目录段，可直接修改。
      </p>
    </div>
    <div class="space-y-2">
      <Label for="worktree-base">起始提交</Label>
      <Input
        id="worktree-base"
        value={baseLabel}
        oninput={(event) => {
          const value = event.currentTarget.value;
          baseEditVersion += 1;
          baseSource = "";
          baseSubject = "";
          upstream = "";
          lastRefreshedAt = null;
          base =
            baseChoices.find((item) => item.label === value)?.value ?? value;
        }}
        list="worktree-bases"
        required
        placeholder="选择提交，或输入分支名 / 提交 ID"
        aria-describedby="worktree-base-help"
        autocomplete="off"
        autocapitalize="off"
        autocorrect="off"
        spellcheck={false}
      />
      <datalist id="worktree-bases"
        >{#each baseChoices as item (item.value)}<option value={item.label}
          ></option>{/each}</datalist
      >
      <p id="worktree-base-help" class="text-xs text-muted-foreground">
        从这条提交创建新分支；也可直接输入本地分支、远程跟踪引用或提交 ID。
      </p>
      {#if data && !baseChoices.length}<p class="text-sm text-muted-foreground">
          尚无可用提交。请先在终端创建提交；仍可在主工作区启动会话。
        </p>{/if}
    </div>
  {:else}
    <div class="space-y-2">
      <Label for="existing-branch">已有本地分支</Label>
      <Command.Root
        label="已有本地分支"
        variant="outline"
        class="h-auto min-w-0"
        filter={(value, search) =>
          value.toLowerCase().includes(search.trim().toLowerCase()) ? 1 : 0}
        disableInitialScroll
      >
        <Command.Input
          id="existing-branch"
          placeholder="搜索本地分支…"
          aria-describedby="existing-branch-selection"
          disabled={disabled || loading || !data}
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
        />
        <ScrollArea
          type="always"
          class="h-48 min-w-0 overflow-hidden"
          scrollbarYClasses="bg-muted/50 [&_[data-slot=scroll-area-thumb]]:bg-muted-foreground"
        >
          <Command.List
            aria-label="本地分支列表"
            class="max-h-none overflow-visible pr-3"
          >
            <Command.Empty>
              {loading
                ? "正在读取本地分支…"
                : error
                  ? "无法读取本地分支"
                  : "没有匹配的本地分支"}
            </Command.Empty>
            {#each data?.branches || [] as item (item.name)}
              <Command.Item
                value={item.name}
                disabled={disabled || item.worktreePaths.length > 0}
                data-checked={branch === item.name}
                onSelect={() => {
                  branch = item.name;
                }}
              >
                <span class="min-w-0 flex-1 break-all">
                  {item.name}
                  {#if item.worktreePaths.length}
                    <span class="block text-muted-foreground"
                      >已被工作树占用</span
                    >
                  {/if}
                </span>
              </Command.Item>
            {/each}
          </Command.List>
        </ScrollArea>
      </Command.Root>
      <p
        id="existing-branch-selection"
        role="status"
        class="break-all text-xs text-muted-foreground"
      >
        {branch
          ? `已选择：${branch}`
          : "搜索后点击分支，或用 ↑↓ 和 Enter 选择未被占用的分支。"}
      </p>
    </div>
  {/if}
  <div class="space-y-2">
    <Label for="worktree-path">新工作树完整路径</Label>
    <Input
      id="worktree-path"
      bind:value={path}
      oninput={() => {
        pathEdited = true;
      }}
      required
      autocomplete="off"
      autocapitalize="off"
      autocorrect="off"
      spellcheck={false}
    />
    <p
      class="font-mono text-xs break-all whitespace-pre-wrap text-muted-foreground"
    >
      {path}
    </p>
    <p class="text-xs text-muted-foreground">
      默认放在仓库旁，目录名取分支最后一段；可以修改完整路径，不覆盖已有目录。
    </p>
  </div>
  <p class="text-xs text-muted-foreground">
    仅检出版本控制内容；只会在刷新起始分支上游时 fetch，不会复制 .env
    或安装依赖。创建成功后即使命令启动失败，工作树与分支仍保留。
  </p>
</div>
