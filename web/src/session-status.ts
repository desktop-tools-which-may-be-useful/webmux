import type { SessionInfo } from "./schemas";

export type SessionState = SessionInfo["status"];

const urgentAttentionKinds = new Set<
  NonNullable<SessionInfo["attention"]>["kind"]
>(["question", "permission", "error"]);

export function hasUrgentAttention(
  session: Pick<SessionInfo, "attention">,
): boolean {
  return (
    session.attention !== null &&
    urgentAttentionKinds.has(session.attention.kind)
  );
}

export type SessionStatusTone =
  | "destructive"
  | "warning"
  | "success"
  | "secondary";

export function sessionStatusTone(
  session: Pick<SessionInfo, "status" | "exitCode" | "attention">,
): SessionStatusTone {
  if (session.status === "exited") {
    return session.exitCode !== null && session.exitCode !== 0
      ? "destructive"
      : "secondary";
  }
  if (hasUrgentAttention(session)) return "destructive";
  if (session.status === "waiting") return "warning";
  if (session.status === "running") return "success";
  return "secondary";
}

export interface SessionCounts {
  // "Running" in list summaries covers both busy and quiet live sessions.
  active: number;
  waiting: number;
  exited: number;
}

export function isActive(status: SessionState) {
  return status === "running" || status === "idle";
}

// "archived" keeps only archived hybrid records, so no live session matches.
export type StatusFilter = "all" | "active" | "waiting" | "exited" | "archived";

export function matchesStatusFilter(
  status: SessionState,
  filter: StatusFilter,
) {
  if (filter === "all") return true;
  if (filter === "archived") return false;
  return filter === "active" ? isActive(status) : status === filter;
}

export function countSessions(
  sessions: readonly Pick<SessionInfo, "status">[],
): SessionCounts {
  const counts: SessionCounts = { active: 0, waiting: 0, exited: 0 };
  for (const { status } of sessions) {
    if (isActive(status)) counts.active++;
    else if (status === "waiting") counts.waiting++;
    else if (status === "exited") counts.exited++;
  }
  return counts;
}

export function statusSummary(
  sessions: readonly Pick<SessionInfo, "status">[],
) {
  const { active, waiting, exited } = countSessions(sessions);
  return `运行 ${active} · 等待 ${waiting} · 退出 ${exited}`;
}

// The four states a sidebar row shows (chat history and the workbench tree):
// waiting for the user, working, idle, or ended (exited, or an archived
// record). Rows always pair the colour with the label.
export type RowStatus = "working" | "waiting" | "idle" | "ended";
export const rowStatusLabels: Record<RowStatus, string> = {
  working: "工作中",
  waiting: "等待",
  idle: "空闲",
  ended: "已结束",
};
export function rowStatus(
  session: Pick<SessionInfo, "status" | "attention" | "agent">,
): RowStatus {
  if (session.status === "exited") return "ended";
  const kind = session.attention?.kind;
  if (
    kind === "question" ||
    kind === "permission" ||
    session.status === "waiting"
  )
    return "waiting";
  if (session.agent?.work || session.status === "running") return "working";
  return "idle";
}
