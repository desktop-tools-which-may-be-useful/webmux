<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconPencil from "~icons/tabler/pencil";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { ListButton } from "$lib/components/ui/list-button";
  import LaunchConfigIcon from "../../src/components/LaunchConfigIcon.svelte";
  import { chats, liveEntry, statusLabels } from "./data";
  import StatusText from "./StatusText.svelte";
  import { titleOf } from "./titles.svelte";

  // Prototype of the chat-mode sidebar content: live chats flat in a stable
  // order (newest created first), with archived records folded below them. The current row uses
  // the navigation variant's line marker; the implementation keys it on
  // aria-current (design.md §8). Live rows offer the existing rename dialog;
  // archived ones do not.
  let {
    current,
    onopen,
    onrename,
  }: {
    current: string | null;
    onopen: (id: string) => void;
    onrename: (id: string) => void;
  } = $props();
  const live = $derived(chats.filter((chat) => liveEntry(chat.id)));
  const archived = $derived(chats.filter((chat) => !liveEntry(chat.id)));
  let archivedOpen = $state(false);
</script>

<h2 class="px-3 pt-3 pb-1 text-xs text-muted-foreground">最近</h2>
<ul class="space-y-0.5" aria-label="聊天历史">
  {#each live as chat (chat.id)}
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
        <span class="min-w-0 flex-1 truncate">{title}</span>
        <StatusText status={chat.status} />
      </ListButton>
      <ListButton
        variant="file-tree-action"
        aria-label={`修改标题：${title}`}
        title={`修改标题：${title}`}
        onclick={() => onrename(chat.id)}
        ><IconPencil aria-hidden="true" /></ListButton
      >
    </li>
  {/each}
</ul>
{#if archived.length}
  <Collapsible.Root bind:open={archivedOpen} class="mt-2">
    <Collapsible.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          variant="ghost"
          size="sm"
          class="w-full justify-start"
        >
          <IconChevronDown
            aria-hidden="true"
            class={archivedOpen ? "" : "-rotate-90"}
          />
          <span class="font-medium text-muted-foreground">已归档</span>
          <Badge variant="secondary" class="ml-auto">{archived.length}</Badge>
        </Button>
      {/snippet}
    </Collapsible.Trigger>
    <Collapsible.Content forceMount hidden={!archivedOpen} class="mt-0.5">
      <ul class="space-y-0.5" aria-label="已归档聊天">
        {#each archived as chat (chat.id)}
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
              <span class="min-w-0 flex-1 truncate text-muted-foreground"
                >{title}</span
              >
              <StatusText status={chat.status} />
            </ListButton>
          </li>
        {/each}
      </ul>
    </Collapsible.Content>
  </Collapsible.Root>
{/if}
