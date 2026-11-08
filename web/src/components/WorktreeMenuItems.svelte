<script lang="ts">
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

  // The worktree-level entries of a "⋯" menu: home-page pinning and removal.
  let {
    pinned,
    ontogglepin,
    onremove,
  }: {
    pinned?: boolean;
    ontogglepin?: () => void;
    onremove?: () => void;
  } = $props();
</script>

{#if ontogglepin}<DropdownMenu.Item onSelect={ontogglepin}
    >{pinned ? "从主页移除" : "添加到主页"}</DropdownMenu.Item
  >{/if}
{#if onremove}<DropdownMenu.Item variant="destructive" onSelect={onremove}
    >删除 worktree</DropdownMenu.Item
  >{/if}
