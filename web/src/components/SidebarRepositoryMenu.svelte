<script lang="ts">
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import SidebarRowMenu from "./SidebarRowMenu.svelte";

  // The "⋯" action menu on an available repository row in the workbench tree:
  // creating a worktree, choosing which ones the home page lists, batch worktree management and the repository details.
  let {
    label,
    tabindex,
    onnewworktree,
    onhome,
    onmanage,
    ondetails,
  }: {
    label: string;
    tabindex?: number;
    onnewworktree: () => void;
    onhome: () => void;
    onmanage: () => void;
    ondetails: () => void;
  } = $props();
</script>

<SidebarRowMenu ariaLabel={`仓库操作：${label}`} {tabindex}>
  <DropdownMenu.Item onSelect={onnewworktree}>新建工作树…</DropdownMenu.Item>
  <DropdownMenu.Item onSelect={onhome}>主页显示…</DropdownMenu.Item>
  <DropdownMenu.Item onSelect={onmanage}>管理工作树…</DropdownMenu.Item>
  <DropdownMenu.Item onSelect={ondetails}>仓库详情…</DropdownMenu.Item>
</SidebarRowMenu>
