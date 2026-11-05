<script lang="ts">
  import { untrack, type Snippet } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { Button } from "$lib/components/ui/button";
  import { copyText } from "../clipboard";
  import { errorMessage } from "../error";
  import {
    addForward,
    forwardLink,
    forwards,
    ignorePort,
    isIgnored,
    pruneIgnored,
  } from "../forwards.svelte";
  import type { ListeningPort } from "../schemas";

  // Strips for the ports this session listens on that are neither forwarded
  // nor ignored: at most two, then one summary that opens the dialog.
  let {
    session,
    listening,
    onshowall,
  }: {
    /** The session ID the ports belong to (the source of new forwards). */
    session: string;
    /**
     * The session's current listening ports; undefined while they are not
     * known (before the session info arrives, or off the terminal side).
     */
    listening: readonly ListeningPort[] | undefined;
    /** Opens the port-forwarding dialog. */
    onshowall?: () => void;
  } = $props();

  // Forwards made with "转发", announced until "知道了"; in memory only.
  let announced = $state.raw<{ port: number; path: string }[]>([]);
  let failed = $state<Record<number, string>>({});
  let copied = $state<number | null>(null);
  const busy = new SvelteSet<number>();

  // A port that stops listening leaves the ignore set, so a restarted server
  // prompts again. Unknown ports prune nothing: a remount must not forget
  // what the user ignored before the session info arrives.
  $effect(() => {
    const ports = listening;
    if (ports) untrack(() => pruneIgnored(session, ports));
  });

  const forwarded = $derived(new Set(forwards.list.map((entry) => entry.port)));
  // Until the forward list is known, any port might be forwarded already.
  const prompts = $derived(
    forwards.ready && listening
      ? listening
          .filter(
            (entry) =>
              !forwarded.has(entry.port) && !isIgnored(session, entry.port),
          )
          .sort((a, b) => a.port - b.port)
      : [],
  );

  function forward(entry: ListeningPort, open: boolean) {
    // A tab opened after the request returns is no longer inside the click,
    // and browsers block it; open a blank one now and point it there later.
    const tab = open ? window.open("", "_blank") : null;
    void create(entry, open, tab);
  }
  async function create(
    entry: ListeningPort,
    open: boolean,
    tab: Window | null,
  ) {
    if (busy.has(entry.port)) {
      tab?.close();
      return;
    }
    busy.add(entry.port);
    delete failed[entry.port];
    try {
      const created = await addForward({
        port: entry.port,
        session,
        process: entry.process,
      });
      if (open && tab) {
        tab.opener = null;
        // Absolute: the blank tab has no address of its own to resolve from.
        tab.location.href = forwardLink(created.path);
      } else {
        // Also after a blocked pop-up, so the forward can still be opened.
        announced = [
          ...announced.filter((item) => item.port !== created.port),
          { port: created.port, path: created.path },
        ];
      }
    } catch (err) {
      tab?.close();
      failed[entry.port] = `转发失败：${errorMessage(err)}`;
    } finally {
      busy.delete(entry.port);
    }
  }
  async function copy(port: number, path: string) {
    copied = (await copyText(forwardLink(path))) ? port : null;
  }
</script>

{#snippet strip(text: string, actions: Snippet, error = "")}
  <div
    class="flex shrink-0 flex-wrap items-center gap-x-2 gap-y-1 border-b bg-muted px-4 py-1.5 text-sm"
    role="status"
  >
    <div class="min-w-0 flex-1 wrap-anywhere max-sm:basis-full">
      <p>{text}</p>
      {#if error}<p class="text-xs text-destructive" role="alert">
          {error}
        </p>{/if}
    </div>
    {@render actions()}
  </div>
{/snippet}

{#each announced as item (item.port)}
  {#snippet done()}
    <Button
      href={item.path}
      target="_blank"
      rel="noopener"
      variant="outline"
      size="sm"
      class="any-pointer-coarse:min-h-11">打开</Button
    >
    <Button
      variant="ghost"
      size="sm"
      class="any-pointer-coarse:min-h-11"
      onclick={() => void copy(item.port, item.path)}
      >{copied === item.port ? "已复制" : "复制链接"}</Button
    >
    <Button
      variant="ghost"
      size="sm"
      class="any-pointer-coarse:min-h-11"
      onclick={() =>
        (announced = announced.filter((entry) => entry.port !== item.port))}
      >知道了</Button
    >
  {/snippet}
  {@render strip(`端口 ${item.port} 已转发到 ${item.path}`, done)}
{/each}

{#each prompts.slice(0, 2) as entry (entry.port)}
  {#snippet choose()}
    <Button
      variant="outline"
      size="sm"
      class="any-pointer-coarse:min-h-11"
      disabled={busy.has(entry.port)}
      onclick={() => forward(entry, true)}>转发并打开</Button
    >
    <Button
      variant="ghost"
      size="sm"
      class="any-pointer-coarse:min-h-11"
      disabled={busy.has(entry.port)}
      onclick={() => forward(entry, false)}>转发</Button
    >
    <Button
      variant="ghost"
      size="sm"
      class="any-pointer-coarse:min-h-11"
      onclick={() => ignorePort(session, entry.port)}>忽略</Button
    >
  {/snippet}
  {@render strip(
    entry.process
      ? `检测到端口 ${entry.port}（${entry.process}）正在监听`
      : `检测到端口 ${entry.port} 正在监听`,
    choose,
    failed[entry.port] ?? "",
  )}
{/each}

{#if prompts.length > 2}
  {#snippet more()}
    {#if onshowall}<Button
        variant="ghost"
        size="sm"
        class="any-pointer-coarse:min-h-11"
        onclick={() => onshowall()}>查看全部</Button
      >{/if}
  {/snippet}
  {@render strip(`另有 ${prompts.length - 2} 个端口`, more)}
{/if}
