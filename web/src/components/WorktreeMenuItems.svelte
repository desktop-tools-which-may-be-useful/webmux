<script lang="ts">
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

  // The worktree-level entries of a "⋯" menu, the same on the home page, in
  // the sidebar and in the repository panel: details, home-page pinning and
  // removal. Each place passes the actions it offers.
  let {
    ondetails,
    pinned,
    ontogglepin,
    onremove,
  }: {
    ondetails?: () => void;
    pinned?: boolean;
    ontogglepin?: () => void;
    onremove?: () => void;
  } = $props();
</script>

{#if ondetails}<DropdownMenu.Item onSelect={ondetails}
    >工作树详情…</DropdownMenu.Item
  >{/if}
{#if ontogglepin}<DropdownMenu.Item onSelect={ontogglepin}
    >{pinned ? "从主页移除" : "添加到主页"}</DropdownMenu.Item
  >{/if}
{#if onremove}<DropdownMenu.Item variant="destructive" onSelect={onremove}
    >删除 worktree…</DropdownMenu.Item
  >{/if}
