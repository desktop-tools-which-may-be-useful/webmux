<script lang="ts">
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import SidebarRowMenu from "./SidebarRowMenu.svelte";

  // The "⋯" action menu on a repository row in the workbench tree: creating a
  // worktree, choosing which ones the home page lists, batch worktree
  // management and the repository details. An unavailable repository offers
  // only its details, where it can be removed from the list.
  let {
    label,
    tabindex,
    unavailable = false,
    onnewworktree,
    onhome,
    onmanage,
    ondetails,
  }: {
    label: string;
    tabindex?: number;
    unavailable?: boolean;
    onnewworktree: () => void;
    onhome: () => void;
    onmanage: () => void;
    ondetails: () => void;
  } = $props();
</script>

<SidebarRowMenu ariaLabel={`仓库操作：${label}`} {tabindex}>
  {#if !unavailable}
    <DropdownMenu.Item onSelect={onnewworktree}>新建工作树…</DropdownMenu.Item>
    <DropdownMenu.Item onSelect={onhome}>主页显示…</DropdownMenu.Item>
    <DropdownMenu.Item onSelect={onmanage}>管理工作树…</DropdownMenu.Item>
  {/if}
  <DropdownMenu.Item onSelect={ondetails}>仓库详情…</DropdownMenu.Item>
</SidebarRowMenu>
