<script lang="ts">
  import { onMount } from "svelte";
  import IconMoon from "~icons/tabler/moon";
  import IconFolderPlus from "~icons/tabler/folder-plus";
  import IconSettings from "~icons/tabler/settings";
  import IconSun from "~icons/tabler/sun";
  import { Button } from "$lib/components/ui/button";
  import { toggleTheme } from "../theme";
  import type { Connection } from "../ws";
  import ForwardButton from "./ForwardButton.svelte";

  // The sidebar's global footer, shared by both modes: the connection and
  // host, adding a repository, port forwarding, settings and the theme. A
  // healthy connection is only the host and a dot; reconnecting says so in
  // words.
  let {
    connection,
    hostname,
    setupSettled,
    rail = false,
    onforward,
    onadd,
    canAdd = true,
  }: {
    connection: Connection;
    hostname: string;
    setupSettled: boolean;
    /** The collapsed rail stacks the buttons and drops the host text. */
    rail?: boolean;
    onforward: () => void;
    /** Opens the add-repository dialog. */
    onadd: () => void;
    /** Git and the registry can take a new repository. */
    canAdd?: boolean;
  } = $props();

  let dark = $state(document.documentElement.classList.contains("dark"));
  const statusLabel = $derived(
    connection === "connected"
      ? hostname
        ? `${hostname} 在线`
        : "在线"
      : connection === "reconnecting"
        ? "连接已断开，重连中…"
        : "正在连接",
  );
  onMount(() => {
    const theme = new MutationObserver(() => {
      dark = document.documentElement.classList.contains("dark");
    });
    theme.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });
    return () => theme.disconnect();
  });
</script>

<div
  class={rail
    ? "flex flex-col items-center gap-1 border-t py-2"
    : "flex items-center gap-1 border-t px-3 py-1.5"}
>
  <span
    class={rail
      ? "flex size-8 items-center justify-center"
      : "flex h-5 min-w-0 flex-1 items-center gap-2 text-xs text-muted-foreground"}
    role="status"
    title={statusLabel}
    ><span
      class={`status-dot shrink-0 ${connection === "connected" ? "status-running" : "status-waiting"}`}
    ></span>{#if rail}<span class="sr-only">{statusLabel}</span
      >{:else if connection === "reconnecting"}<span
        class="truncate text-warning">连接已断开，重连中…</span
      >{:else if connection === "connected" && setupSettled}<span
        class="truncate">{hostname || "在线"}</span
      >{:else}<span class="sr-only">{statusLabel}</span>{/if}</span
  >
  <Button
    variant="ghost"
    size="icon-touch"
    aria-label="添加仓库"
    title="添加仓库"
    disabled={!canAdd}
    onclick={() => onadd()}><IconFolderPlus aria-hidden="true" /></Button
  >
  <ForwardButton onclick={onforward} />
  <Button
    href="/settings"
    variant="ghost"
    size="icon-touch"
    aria-label="设置"
    title="设置"><IconSettings aria-hidden="true" /></Button
  >
  <Button
    variant="ghost"
    size="icon-touch"
    class="shrink-0"
    aria-label={dark ? "切换为白色主题" : "切换为黑色主题"}
    title={dark ? "切换为白色主题" : "切换为黑色主题"}
    onclick={toggleTheme}
    >{#if dark}<IconSun aria-hidden="true" />{:else}<IconMoon
        aria-hidden="true"
      />{/if}</Button
  >
</div>
