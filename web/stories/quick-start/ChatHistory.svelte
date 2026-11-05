<script lang="ts">
  import IconPencil from "~icons/tabler/pencil";
  import { ListButton } from "$lib/components/ui/list-button";
  import LaunchConfigIcon from "../../src/components/LaunchConfigIcon.svelte";
  import { chats, liveEntry, statusLabels } from "./data";
  import StatusText from "./StatusText.svelte";
  import { titleOf } from "./titles.svelte";

  // Prototype of the chat-mode sidebar content: every chat session flat by
  // latest activity. The current row uses the navigation variant's line
  // marker; the implementation keys it on aria-current (design.md §8). Live
  // rows offer the existing rename dialog; archived ones do not.
  let {
    current,
    onopen,
    onrename,
  }: {
    current: string | null;
    onopen: (id: string) => void;
    onrename: (id: string) => void;
  } = $props();
</script>

<h2 class="px-3 pt-3 pb-1 text-xs text-muted-foreground">最近</h2>
<ul class="space-y-0.5" aria-label="聊天历史">
  {#each chats as chat (chat.id)}
    {@const title = titleOf(chat)}
    <li class="group/row relative">
      <ListButton
        variant="navigation"
        class="pr-9"
        aria-pressed={chat.id === current}
        title={`${title}（${chat.agentTitle} · ${statusLabels[chat.status]}）`}
        onclick={() => onopen(chat.id)}
      >
        <LaunchConfigIcon icon={chat.icon} size="sm" />
        <span
          class={`min-w-0 flex-1 truncate ${chat.status === "ended" ? "text-muted-foreground" : ""}`}
          >{title}</span
        >
        <StatusText status={chat.status} />
      </ListButton>
      {#if liveEntry(chat.id)}
        <ListButton
          variant="file-tree-action"
          aria-label={`修改标题：${title}`}
          title={`修改标题：${title}`}
          onclick={() => onrename(chat.id)}
          ><IconPencil aria-hidden="true" /></ListButton
        >
      {/if}
    </li>
  {/each}
</ul>
