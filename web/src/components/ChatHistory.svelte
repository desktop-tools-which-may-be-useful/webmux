<script lang="ts" module>
  import type {
    HybridRecord,
    LaunchConfigSummary,
    SessionInfo,
  } from "../schemas";

  export type ChatEntry =
    | { kind: "live"; id: string; at: number; session: SessionInfo }
    | { kind: "archived"; id: string; at: number; record: HybridRecord };

  /**
   * Every chat session, newest activity first: live hybrid sessions and
   * archived records. Terminal-only sessions are left out, and a live
   * session wins over a record with its ID.
   */
  export function chatEntries(
    sessions: readonly SessionInfo[],
    records: readonly HybridRecord[],
  ): ChatEntry[] {
    const live: ChatEntry[] = sessions
      .filter((session) => session.hybrid?.state === "active")
      .map((session) => ({
        kind: "live",
        id: session.id,
        at: Date.parse(session.lastActiveAt),
        session,
      }));
    const ids = new Set(live.map((entry) => entry.id));
    const archived: ChatEntry[] = records
      .filter((record) => !ids.has(record.id))
      .map((record) => ({
        kind: "archived",
        id: record.id,
        at: Date.parse(record.updatedAt),
        record,
      }));
    return [...live, ...archived].sort((a, b) => b.at - a.at);
  }
</script>

<script lang="ts">
  import { ListButton } from "$lib/components/ui/list-button";
  import { rowStatus, rowStatusLabels } from "../session-status";
  import LaunchConfigIcon from "./LaunchConfigIcon.svelte";
  import RowStatus from "./RowStatus.svelte";
  import SidebarSessionMenu from "./SidebarSessionMenu.svelte";

  // The chat-mode sidebar: every chat flat by latest activity. The current
  // row carries aria-current and every row ends in a status dot; the "⋯"
  // menu offers a live chat's rename and close, and an archived one's
  // resume and record deletion. Archived rows also resume when clicked.
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
    records: readonly HybridRecord[];
    configs: readonly LaunchConfigSummary[];
    /** The session page shown, or "" on a home page. */
    current: string;
    /** The record being resumed; its row shows the progress. */
    resuming?: string;
    onopen: (session: SessionInfo) => void;
    onresume: (record: HybridRecord) => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
    /** Asks to delete an archived record. */
    onremove: (record: HybridRecord) => void;
  } = $props();
  const entries = $derived(chatEntries(sessions, records));
</script>

<h2 class="px-3 pt-1 pb-1 text-xs text-muted-foreground">最近</h2>
{#if entries.length}
  <ul class="space-y-0.5" aria-label="聊天历史">
    {#each entries as entry (entry.id)}
      {#if entry.kind === "live"}
        {@const session = entry.session}
        {@const status = rowStatus(session)}
        <li class="group/row relative">
          <ListButton
            variant="navigation-action"
            aria-current={session.id === current ? "page" : undefined}
            aria-label={`${session.title}（${rowStatusLabels[status]}）`}
            title={`${session.title}（${session.hybrid?.agentTitle ?? ""} · ${rowStatusLabels[status]}）`}
            onclick={() => onopen(session)}
          >
            <LaunchConfigIcon
              icon={session.hybrid?.agentIcon ?? ""}
              size="sm"
            />
            <span class="min-w-0 flex-1 truncate">{session.title}</span>
            <RowStatus {status} />
          </ListButton>
          <SidebarSessionMenu
            label={session.title}
            target={{ kind: "live", session, onrename, onclose }}
          />
        </li>
      {:else}
        {@const record = entry.record}
        {@const title = record.name || record.title || record.agent}
        <li class="group/row relative">
          <!-- The same end padding as live rows keeps every status in one
               column. -->
          <ListButton
            variant="navigation-action"
            aria-current={record.id === current ? "page" : undefined}
            aria-label={`${title}（${resuming === record.id ? "正在恢复" : "已结束"}）`}
            title={`${title}（${record.agent} · 已结束，点击恢复）`}
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
      {/if}
    {/each}
  </ul>
{:else}
  <p class="px-3 py-2 text-sm text-muted-foreground">
    还没有聊天。写下任务，发送后就会出现在这里。
  </p>
{/if}
