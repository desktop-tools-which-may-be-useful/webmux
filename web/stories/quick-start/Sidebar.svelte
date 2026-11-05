<script lang="ts">
  import IconLayoutDashboard from "~icons/tabler/layout-dashboard";
  import IconLayoutSidebarLeftCollapse from "~icons/tabler/layout-sidebar-left-collapse";
  import IconMessageCircle from "~icons/tabler/message-circle";
  import IconMessagePlus from "~icons/tabler/message-plus";
  import IconMoon from "~icons/tabler/moon";
  import IconPlus from "~icons/tabler/plus";
  import IconSettings from "~icons/tabler/settings";
  import IconSun from "~icons/tabler/sun";
  import { Button } from "$lib/components/ui/button";
  import Brand from "../../src/components/Brand.svelte";
  import ChatHistory from "./ChatHistory.svelte";
  import type { Mode } from "./data";
  import WorkbenchTree from "./WorkbenchTree.svelte";

  // Prototype of the app sidebar: brand and the two mode buttons on top,
  // the mode's primary action, its navigation, then the global footer
  // (connection, settings, theme) that replaces the home header.
  let {
    mode,
    scope,
    current,
    onmode,
    onnew,
    onscope,
    onopen,
    onnewworktree,
    onrename,
    oncollapse,
  }: {
    mode: Mode;
    scope: string;
    current: string | null;
    onmode: (mode: Mode) => void;
    onnew: () => void;
    onscope: (scope: string, worktree?: string) => void;
    onopen: (id: string) => void;
    onnewworktree: (repositoryId: string) => void;
    onrename: (id: string) => void;
    /** Desktop only; the drawer closes on its own. */
    oncollapse?: () => void;
  } = $props();
  let dark = $state(document.documentElement.classList.contains("dark"));
  function toggleTheme() {
    dark = !dark;
    document.documentElement.classList.toggle("dark", dark);
  }
</script>

<nav class="flex h-full min-h-0 flex-col" aria-label="导航">
  <div class="flex items-center gap-1 px-2 pt-2 pb-1">
    {#if oncollapse}
      <Button
        variant="ghost"
        size="icon-touch"
        aria-label="收起侧栏"
        title="收起侧栏"
        onclick={oncollapse}
        ><IconLayoutSidebarLeftCollapse aria-hidden="true" /></Button
      >
    {/if}
    <span class="brand mr-auto px-1"><Brand /></span>
    <div class="flex items-center" role="group" aria-label="模式">
      <Button
        variant="ghost"
        size="icon-touch"
        aria-label="聊天模式"
        aria-pressed={mode === "chat"}
        title="聊天模式：快速开始与聊天历史"
        onclick={() => onmode("chat")}
        ><IconMessageCircle aria-hidden="true" /></Button
      >
      <Button
        variant="ghost"
        size="icon-touch"
        aria-label="工作台模式"
        aria-pressed={mode === "workbench"}
        title="工作台模式：仓库、worktree 与全部会话"
        onclick={() => onmode("workbench")}
        ><IconLayoutDashboard aria-hidden="true" /></Button
      >
    </div>
  </div>
  <div class="px-2 pt-2">
    <Button variant="outline" size="sm-touch" class="w-full" onclick={onnew}>
      {#if mode === "chat"}<IconMessagePlus
          aria-hidden="true"
        />新聊天{:else}<IconPlus aria-hidden="true" />新建会话{/if}
    </Button>
  </div>
  <div class="min-h-0 flex-1 overflow-y-auto px-2 py-2">
    {#if mode === "chat"}
      <ChatHistory {current} {onopen} {onrename} />
    {:else}
      <WorkbenchTree
        {scope}
        {current}
        {onscope}
        {onopen}
        {onnewworktree}
        {onrename}
      />
    {/if}
  </div>
  <div class="flex items-center gap-1 border-t px-3 py-1.5">
    <span
      class="flex min-w-0 flex-1 items-center gap-2 text-xs text-muted-foreground"
      title="已连接 devbox"
      ><span class="status-dot status-running"></span><span class="truncate"
        >devbox</span
      ></span
    >
    <Button variant="ghost" size="icon-touch" aria-label="设置" title="设置"
      ><IconSettings aria-hidden="true" /></Button
    >
    <Button
      variant="ghost"
      size="icon-touch"
      aria-label={dark ? "切换为白色主题" : "切换为黑色主题"}
      title={dark ? "切换为白色主题" : "切换为黑色主题"}
      onclick={toggleTheme}
      >{#if dark}<IconSun aria-hidden="true" />{:else}<IconMoon
          aria-hidden="true"
        />{/if}</Button
    >
  </div>
</nav>
