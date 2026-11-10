<script lang="ts">
  import { onDestroy } from "svelte";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { ListButton } from "$lib/components/ui/list-button";
  import { relativeTime } from "../commit-choices";
  import { errorMessage } from "../error";
  import { LatestRequest } from "../hooks/remote-request.svelte";
  import type { Branches } from "../schemas";
  import { upstreamState } from "../worktree-sync";
  import InlineNotice from "./InlineNotice.svelte";
  import ListSearchInput from "./ListSearchInput.svelte";
  import UpstreamStatus from "./UpstreamStatus.svelte";

  // The repository panel's branches: local branches with their upstream and
  // the worktree that has each checked out, then the remote-tracking
  // branches in a fold. Read when the section is first shown; both groups
  // filter by name.
  let {
    active,
    load,
    worktreePaths,
    onopen,
  }: {
    /** The section is shown; the first time starts the read. */
    active: boolean;
    load: (signal: AbortSignal) => Promise<Branches>;
    /** Paths of the repository's worktrees; others are shown as text. */
    worktreePaths: ReadonlySet<string>;
    /** Opens the details of the worktree at a path. */
    onopen: (path: string) => void;
  } = $props();

  let data = $state<Branches | null>(null);
  let error = $state("");
  let loading = $state(false);
  let query = $state("");
  let remoteOpen = $state(false);
  const requests = new LatestRequest();
  onDestroy(() => requests.dispose());

  async function read() {
    const request = requests.begin();
    loading = true;
    error = "";
    try {
      const result = await load(request.signal);
      if (request.current()) data = result;
    } catch (err) {
      if (request.reportable()) error = errorMessage(err);
    } finally {
      if (request.current()) loading = false;
    }
  }
  $effect(() => {
    if (active && !data && !loading && !error) void read();
  });

  const remotePrefix = "refs/remotes/";
  const term = $derived(query.trim().toLowerCase());
  const local = $derived(data?.branches ?? []);
  const remote = $derived(
    (data?.bases ?? [])
      .filter((base) => base.kind === "remote")
      .map((base) => ({
        ...base,
        label: base.name.startsWith(remotePrefix)
          ? base.name.slice(remotePrefix.length)
          : base.name,
      })),
  );
  const localShown = $derived(
    term
      ? local.filter((branch) => branch.name.toLowerCase().includes(term))
      : local,
  );
  const remoteShown = $derived(
    term
      ? remote.filter((branch) => branch.label.toLowerCase().includes(term))
      : remote,
  );
  // A search shows the matching remote branches without another click.
  const remoteExpanded = $derived(remoteOpen || !!term);
  const countText = (shown: number, total: number) =>
    term ? `匹配 ${shown} / ${total}` : String(total);
</script>

<div class="space-y-3">
  <ListSearchInput
    bind:value={query}
    label="搜索分支"
    placeholder="按分支名称搜索…"
  />

  {#if error}
    <InlineNotice tone="danger">无法读取分支：{error}</InlineNotice>
    <Button size="sm" variant="outline" onclick={() => void read()}
      >重新读取</Button
    >
  {:else if !data}
    <p role="status" class="text-xs text-muted-foreground">正在读取分支…</p>
  {:else}
    <section class="space-y-2" aria-label="本地分支">
      <h3 class="flex items-center gap-2 text-sm font-medium">
        本地分支
        <Badge variant="secondary"
          >{countText(localShown.length, local.length)}</Badge
        >
      </h3>
      {#if localShown.length === 0}
        <p class="text-xs text-muted-foreground">
          {local.length ? "没有匹配的本地分支。" : "没有本地分支。"}
        </p>
      {:else}
        <ul class="space-y-2">
          {#each localShown as branch (branch.name)}
            <li class="space-y-1 rounded-lg border p-3 text-xs">
              <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                <span class="font-mono text-sm font-medium break-all"
                  >{branch.name}</span
                >
                <span class="font-mono text-muted-foreground"
                  >{branch.head.slice(0, 8)}</span
                >
                {#if branch.name === data.defaultBranch}<Badge
                    variant="secondary">默认</Badge
                  >{/if}
              </div>
              <p class="break-all text-muted-foreground">
                {branch.subject || "无标题提交"}{branch.committedAt
                  ? ` · ${relativeTime(branch.committedAt)}`
                  : ""}
              </p>
              <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                <UpstreamStatus state={upstreamState(branch)} />
              </div>
              {#each branch.worktreePaths as path (path)}
                <p class="flex min-w-0 flex-wrap items-baseline gap-1">
                  <span class="text-muted-foreground">检出于</span>
                  {#if worktreePaths.has(path)}
                    <ListButton
                      variant="code-link"
                      aria-haspopup="dialog"
                      title={`打开工作树详情：${path}`}
                      onclick={() => onopen(path)}>{path}</ListButton
                    >
                  {:else}<span class="font-mono break-all">{path}</span>{/if}
                </p>
              {/each}
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <Collapsible.Root
      open={remoteExpanded}
      onOpenChange={(value) => (remoteOpen = value)}
    >
      <section class="space-y-2" aria-label="远程分支">
        <Collapsible.Trigger disabled={!!term}>
          {#snippet child({ props })}
            <Button {...props} variant="ghost" size="sm" class="-ml-2">
              <IconChevronDown
                aria-hidden="true"
                class={remoteExpanded ? "" : "-rotate-90"}
              />
              <span class="font-medium">远程分支</span>
              <Badge variant="secondary"
                >{countText(remoteShown.length, remote.length)}</Badge
              >
            </Button>
          {/snippet}
        </Collapsible.Trigger>
        <Collapsible.Content forceMount hidden={!remoteExpanded}>
          {#if remoteShown.length === 0}
            <p class="text-xs text-muted-foreground">
              {remote.length ? "没有匹配的远程分支。" : "没有远程跟踪分支。"}
            </p>
          {:else}
            <ul class="space-y-1">
              {#each remoteShown as branch (branch.name)}
                <li
                  class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5 border-b py-1.5 text-xs last:border-b-0"
                >
                  <span class="font-mono break-all">{branch.label}</span>
                  <span class="font-mono text-muted-foreground"
                    >{branch.head.slice(0, 8)}</span
                  >
                  <span class="min-w-0 break-all text-muted-foreground"
                    >{branch.subject || "无标题提交"}</span
                  >
                </li>
              {/each}
            </ul>
          {/if}
        </Collapsible.Content>
      </section>
    </Collapsible.Root>
  {/if}
</div>
