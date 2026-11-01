<script lang="ts">
  import { errorMessage } from "../error";
  import { onMount } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import IconPictureInPictureOff from "~icons/tabler/picture-in-picture-off";
  import CodeWorkspace from "./CodeWorkspace.svelte";
  import type { CodeHost } from "../code-host";
  import type {
    CodeHostState,
    CodeWorkspaceSnapshot,
    ReportedCwd,
  } from "../schemas";
  import type { TextInsertion } from "../terminal-draft.svelte";
  import { popoutAvailable, popOut, type CodePopout } from "./codePopout";

  let {
    id,
    visible,
    connection,
    paste,
    draftEnabled,
    sessionCwd,
    reportedCwd,
    reveal,
  }: {
    id: string;
    visible: boolean;
    connection: string;
    paste: (text: string) => TextInsertion;
    draftEnabled: boolean;
    sessionCwd: string;
    reportedCwd: ReportedCwd | null;
    /** A path another view asks to show, such as a chat tool call's diff. */
    reveal?: { path: string; key: number };
  } = $props();

  const host: CodeHost = {
    get draftEnabled() {
      return draftEnabled;
    },
    get connection() {
      return connection;
    },
    get sessionCwd() {
      return sessionCwd;
    },
    get reportedCwd() {
      return reportedCwd;
    },
    get reveal() {
      return reveal;
    },
    paste: (text) => Promise.resolve(paste(text)),
  };
  const hostState = $derived<CodeHostState>({
    draftEnabled,
    connection,
    sessionCwd,
    reportedCwd,
  });

  const canPopOut = popoutAvailable();
  let workspace = $state<CodeWorkspace>();
  let wrapper = $state<HTMLDivElement>();
  // While popped out the window's frame owns the workspace; this page keeps
  // only the latest state it reported, and remounts from it on return.
  let popout = $state<CodePopout>();
  // While the window starts, the workspace here is inert: the window restores
  // the state captured when popping began.
  let popping = $state(false);
  let snapshot = $state.raw<CodeWorkspaceSnapshot>();
  let generation = $state(0);
  let notice = $state("");
  let mounted = false;

  async function toggle() {
    if (popout) return popout.close();
    if (!workspace || !wrapper || popping) return;
    popping = true;
    notice = "";
    const current = workspace.capture();
    // Requests this workspace already showed are part of `current`.
    const shownReveal = reveal?.key ?? 0;
    const rect = wrapper.getBoundingClientRect();
    try {
      const opened = await popOut({
        id,
        title: `代码 · ${current.context?.rootPath ?? id}`,
        width: rect.width,
        height: rect.height,
        snapshot: current,
        host: $state.snapshot(hostState),
        paste,
        closed(latest) {
          popout = undefined;
          if (!mounted) return;
          snapshot = latest;
          generation++;
        },
      });
      // A request that arrived while the window started is shown there too.
      forwarded = shownReveal;
      if (mounted) popout = opened;
      else opened.dispose();
    } catch (e) {
      if (mounted) notice = `独立窗口打开失败：${errorMessage(e)}`;
    } finally {
      popping = false;
    }
  }

  $effect(() => {
    const state = $state.snapshot(hostState);
    popout?.host(state);
  });
  let forwarded = 0;
  $effect(() => {
    const target = reveal;
    if (!popout || !target || target.key === forwarded) return;
    forwarded = target.key;
    // Reactive props are proxies, which a message port cannot clone.
    popout.reveal({ path: target.path, key: target.key });
  });

  onMount(() => {
    mounted = true;
    return () => {
      mounted = false;
      popout?.dispose();
    };
  });
</script>

<div
  bind:this={wrapper}
  class="flex h-full min-h-0 min-w-0 flex-col"
  inert={popping}
  aria-busy={popping}
>
  {#if notice}<p
      role="alert"
      class="shrink-0 px-2 py-1 text-sm break-all text-destructive"
    >
      {notice}
    </p>{/if}
  {#if popout}<div
      class="flex flex-1 flex-col items-center justify-center gap-3 p-4 text-sm text-muted-foreground"
    >
      <p>代码已在独立窗口中显示</p>
      <Button variant="outline" size="sm" onclick={() => popout?.close()}
        ><IconPictureInPictureOff aria-hidden="true" />收回到此处</Button
      >
    </div>
  {:else}
    {#key generation}<CodeWorkspace
        bind:this={workspace}
        {id}
        {visible}
        {host}
        initial={snapshot}
        popout={canPopOut
          ? { popped: false, toggle: () => void toggle() }
          : undefined}
      />{/key}
  {/if}
</div>
