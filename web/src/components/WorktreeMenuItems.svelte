<script lang="ts">
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

  // The worktree-level entries of a "⋯" menu, the same on the home page, in
  // the sidebar and in the repository panel: details, switching its branch,
  // home-page pinning and removal. Each place passes the actions it offers;
  // switching is left out for bare, missing and prunable worktrees.
  let {
    ondetails,
    onswitch,
    pinned,
    ontogglepin,
    onremove,
  }: {
    ondetails?: () => void;
    onswitch?: () => void;
    pinned?: boolean;
    ontogglepin?: () => void;
    onremove?: () => void;
  } = $props();
</script>

{#if ondetails}<DropdownMenu.Item onSelect={ondetails}
    >工作树详情…</DropdownMenu.Item
  >{/if}
{#if onswitch}<DropdownMenu.Item onSelect={onswitch}
    >切换分支…</DropdownMenu.Item
  >{/if}
{#if ontogglepin}<DropdownMenu.Item onSelect={ontogglepin}
    >{pinned ? "从主页移除" : "添加到主页"}</DropdownMenu.Item
  >{/if}
{#if onremove}<DropdownMenu.Item variant="destructive" onSelect={onremove}
    >删除 worktree…</DropdownMenu.Item
  >{/if}
