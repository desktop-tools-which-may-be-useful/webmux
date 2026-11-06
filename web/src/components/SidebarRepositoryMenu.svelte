<script lang="ts">
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import SidebarRowMenu from "./SidebarRowMenu.svelte";

  // The "⋯" action menu on an available repository row in the workbench tree:
  // creating a new worktree or opening the batch worktree management dialog.
  let {
    label,
    tabindex,
    onnewworktree,
    onmanage,
  }: {
    label: string;
    tabindex?: number;
    onnewworktree: () => void;
    onmanage: () => void;
  } = $props();
</script>

<SidebarRowMenu ariaLabel={`仓库操作：${label}`} {tabindex}>
  <DropdownMenu.Item onSelect={onnewworktree}>新建工作树…</DropdownMenu.Item>
  <DropdownMenu.Item onSelect={onmanage}>管理工作树…</DropdownMenu.Item>
</SidebarRowMenu>
