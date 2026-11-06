<script lang="ts">
  import IconDots from "~icons/tabler/dots";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { SessionInfo } from "../schemas";

  // The "⋯" button over the end of a sidebar session row and its menu: the
  // row's actions, the same in the chat history and the workbench tree.
  // The button shows while the row is hovered or focused or the menu is
  // open, and always on touch screens, where a tap opens the menu.
  type Target =
    | {
        kind: "live";
        session: SessionInfo;
        onrename: (session: SessionInfo) => void;
        onclose: (session: SessionInfo) => void;
      }
    | {
        kind: "archived";
        onresume: () => void;
        /** Offered only while the record's worktree can be released. */
        onrelease?: () => void;
        onremove: () => void;
      };
  let {
    label,
    target,
    tabindex,
    disabled = false,
  }: {
    /** The row's title, naming the button. */
    label: string;
    target: Target;
    tabindex?: number;
    disabled?: boolean;
  } = $props();
  // The floating layer mounts on first use, as on the home cards.
  let ready = $state(false);
  const closeLabel = $derived.by(() => {
    if (target.kind !== "live") return "";
    const { session } = target;
    if (session.status === "exited") return "移除会话";
    return session.views.includes("chat") && session.state === "active"
      ? "停止会话并归档"
      : "关闭会话";
  });
</script>

<DropdownMenu.Root
  onOpenChange={(open) => {
    if (open) ready = true;
  }}
>
  <DropdownMenu.Trigger {disabled}>
    {#snippet child({ props })}
      <ListButton
        {...props}
        variant="file-tree-action"
        aria-label={`会话操作：${label}`}
        title={`会话操作：${label}`}
        {tabindex}><IconDots aria-hidden="true" /></ListButton
      >
    {/snippet}
  </DropdownMenu.Trigger>
  {#if ready}
    <DropdownMenu.Content align="end" class="min-w-40">
      {#if target.kind === "live"}
        {@const { session, onrename, onclose } = target}
        <DropdownMenu.Item onSelect={() => onrename(session)}
          >修改标题</DropdownMenu.Item
        >
        <DropdownMenu.Separator />
        <DropdownMenu.Item
          variant={closeLabel === "停止会话并归档" ? "default" : "destructive"}
          onSelect={() => onclose(session)}>{closeLabel}</DropdownMenu.Item
        >
      {:else}
        <DropdownMenu.Item onSelect={target.onresume}
          >恢复聊天</DropdownMenu.Item
        >
        {#if target.onrelease}<DropdownMenu.Item onSelect={target.onrelease}
            >释放空间</DropdownMenu.Item
          >{/if}
        <DropdownMenu.Separator />
        <DropdownMenu.Item variant="destructive" onSelect={target.onremove}
          >删除记录</DropdownMenu.Item
        >
      {/if}
    </DropdownMenu.Content>
  {/if}
</DropdownMenu.Root>
