<script lang="ts" module>
  import type {
    ArchivedSession,
    LaunchConfigSummary,
    SessionInfo,
  } from "../schemas";

  /**
   * Live sessions with a chat view and archived sessions, each sorted newest activity
   * first. Terminal-only sessions are left out, and a live session wins over
   * a record with its ID.
   */
  export function chatHistorySections(
    sessions: readonly SessionInfo[],
    records: readonly ArchivedSession[],
  ): { live: SessionInfo[]; archived: ArchivedSession[] } {
    const live = sessions
      .filter(
        (session) =>
          session.views.includes("chat") && session.state === "active",
      )
      .sort((a, b) => Date.parse(b.lastActiveAt) - Date.parse(a.lastActiveAt));
    const ids = new Set(live.map((session) => session.id));
    const archived = records
      .filter((record) => !ids.has(record.id))
      .sort(
        (a, b) =>
          Date.parse(b.archive.updatedAt) - Date.parse(a.archive.updatedAt),
      );
    return { live, archived };
  }
</script>

<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { ListButton } from "$lib/components/ui/list-button";
  import { rowStatus, rowStatusLabels } from "../session-status";
  import LaunchConfigIcon from "./LaunchConfigIcon.svelte";
  import RowStatus from "./RowStatus.svelte";
  import SidebarSessionMenu from "./SidebarSessionMenu.svelte";

  // The chat-mode sidebar: live chats flat by latest activity, with archived
  // records folded below them. The current row carries aria-current and every
  // row ends in a status dot; the "⋯" menu offers a live chat's rename and
  // archive, and an archived one's resume and record deletion. Archived rows
  // also resume when clicked.
  let {
    sessions,
    records,
    configs,
    current,
    resuming = "",
    onopen,
    onresume,
    onrename,
    onclose,
    onremove,
  }: {
    sessions: readonly SessionInfo[];
    records: readonly ArchivedSession[];
    configs: readonly LaunchConfigSummary[];
    /** The session page shown, or "" on a home page. */
    current: string;
    /** The record being resumed; its row shows the progress. */
    resuming?: string;
    onopen: (session: SessionInfo) => void;
    onresume: (record: ArchivedSession) => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
    /** Asks to delete an archived record. */
    onremove: (record: ArchivedSession) => void;
  } = $props();
  const { live, archived } = $derived(chatHistorySections(sessions, records));
  let archivedExpanded = $state(false);
  let archivedOverride = $state<boolean | undefined>();
  const currentInArchived = $derived(
    archived.some((record) => record.id === current || record.id === resuming),
  );
  $effect(() => {
    void currentInArchived;
    archivedOverride = undefined;
  });
  const archivedOpen = $derived(
    archivedOverride ?? (currentInArchived || archivedExpanded),
  );
</script>

<h2 class="px-3 pt-1 pb-1 text-xs text-muted-foreground">最近</h2>
{#if live.length || archived.length}
  {#if live.length}
    <ul class="space-y-0.5" aria-label="聊天历史">
      {#each live as session (session.id)}
        {@const status = rowStatus(session)}
        <li class="group/row relative">
          <ListButton
            variant="navigation-action"
            aria-current={session.id === current ? "page" : undefined}
            aria-label={`${session.title}（${rowStatusLabels[status]}）`}
            title={`${session.title}（${session.agent?.title ?? ""} · ${rowStatusLabels[status]}）`}
            onclick={() => onopen(session)}
          >
            <LaunchConfigIcon icon={session.agent?.icon ?? ""} size="sm" />
            <span class="min-w-0 flex-1 truncate">{session.title}</span>
            <RowStatus {status} />
          </ListButton>
          <SidebarSessionMenu
            label={session.title}
            target={{ kind: "live", session, onrename, onclose }}
          />
        </li>
      {/each}
    </ul>
  {/if}
  {#if archived.length}
    <Collapsible.Root
      open={archivedOpen}
      onOpenChange={(value) => {
        if (currentInArchived) archivedOverride = value;
        else archivedExpanded = value;
      }}
      class={live.length ? "mt-2" : ""}
    >
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
          {#each archived as record (record.id)}
            {@const title = record.title || record.agent.name}
            <li class="group/row relative">
              <!-- The same end padding as live rows keeps every status in one
                   column. -->
              <ListButton
                variant="navigation-action"
                aria-current={record.id === current ? "page" : undefined}
                aria-label={`${title}（${resuming === record.id ? "正在恢复" : "已结束"}）`}
                title={`${title}（${record.agent.name} · 已结束，点击恢复）`}
                aria-busy={resuming === record.id}
                disabled={!!resuming}
                onclick={() => onresume(record)}
              >
                <LaunchConfigIcon
                  icon={configs.find((config) => config.id === record.configId)
                    ?.icon ?? ""}
                  size="sm"
                />
                <span class="min-w-0 flex-1 truncate text-muted-foreground"
                  >{title}</span
                >
                {#if resuming === record.id}<span
                    class="shrink-0 text-xs font-normal text-muted-foreground"
                    >恢复中…</span
                  >{:else}<RowStatus status="ended" />{/if}
              </ListButton>
              <SidebarSessionMenu
                label={title}
                disabled={!!resuming}
                target={{
                  kind: "archived",
                  onresume: () => onresume(record),
                  onremove: () => onremove(record),
                }}
              />
            </li>
          {/each}
        </ul>
      </Collapsible.Content>
    </Collapsible.Root>
  {/if}
{:else}
  <p class="px-3 py-2 text-sm text-muted-foreground">
    还没有聊天。写下任务，发送后就会出现在这里。
  </p>
{/if}
