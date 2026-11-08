<script lang="ts">
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

  // The worktree-level entries of a "⋯" menu: home-page pinning and removal.
  let {
    pinned,
    occupied = false,
    ontogglepin,
    onremove,
  }: {
    pinned?: boolean;
    /**
     * The worktree holds sessions or archived records, which keep it on the
     * home page whatever is pinned; unpinning would visibly do nothing.
     */
    occupied?: boolean;
    ontogglepin?: () => void;
    onremove?: () => void;
  } = $props();
  const locked = $derived(pinned === true && occupied);
</script>

{#if ontogglepin}<DropdownMenu.Item disabled={locked} onSelect={ontogglepin}
    >{pinned ? "从主页移除" : "添加到主页"}</DropdownMenu.Item
  >{#if locked}<DropdownMenu.Label
      >有会话或归档记录，结束后才能移除</DropdownMenu.Label
    >{/if}{/if}
{#if onremove}<DropdownMenu.Item variant="destructive" onSelect={onremove}
    >删除 worktree</DropdownMenu.Item
  >{/if}
