<script lang="ts">
  import IconDots from "~icons/tabler/dots";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import RepositoryMenuItems from "./RepositoryMenuItems.svelte";
  import WorktreeMenuItems from "./WorktreeMenuItems.svelte";

  // The home page's "⋯" menu: the repository in scope and, when a worktree is
  // picked in the tree, that worktree. Phones show it in the top bar, wider
  // screens in the toolbar (see WorkspaceActions).
  let {
    repository,
    unavailable = false,
    onnewworktree,
    onhome,
    onmanage,
    onsettings,
    ondetails,
    worktree,
  }: {
    repository: string;
    unavailable?: boolean;
    onnewworktree: () => void;
    onhome: () => void;
    onmanage: () => void;
    onsettings: () => void;
    ondetails: () => void;
    /** The picked worktree; its entries follow the repository's. */
    worktree?: {
      title: string;
      ondetails: () => void;
      pinned: boolean;
      ontogglepin: () => void;
      onremove?: () => void;
    };
  } = $props();
  const label = $derived(`仓库菜单：${repository}`);
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger>
    {#snippet child({ props })}
      <Button
        {...props}
        variant="ghost"
        size="icon-sm"
        class="min-h-9 min-w-9 sm:min-h-8 sm:min-w-8"
        aria-label={label}
        title={label}><IconDots aria-hidden="true" /></Button
      >
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="end" class="min-w-48">
    <RepositoryMenuItems
      {unavailable}
      {onnewworktree}
      {onhome}
      {onmanage}
      {onsettings}
      {ondetails}
    />
    {#if worktree}
      <DropdownMenu.Separator />
      <DropdownMenu.Label>{worktree.title}</DropdownMenu.Label>
      <WorktreeMenuItems
        ondetails={worktree.ondetails}
        pinned={worktree.pinned}
        ontogglepin={worktree.ontogglepin}
        onremove={worktree.onremove}
      />
    {/if}
  </DropdownMenu.Content>
</DropdownMenu.Root>
