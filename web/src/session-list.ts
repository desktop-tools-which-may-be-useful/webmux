import type { z } from "zod";
import type { Repository, SessionEventSchema, SessionInfo } from "./schemas";

function compare(a: SessionInfo, b: SessionInfo) {
  return (
    Date.parse(b.createdAt) - Date.parse(a.createdAt) ||
    a.id.localeCompare(b.id)
  );
}

/** Keep creation order without sorting the full list on every output update. */
export function applySessionEvent(
  sessions: SessionInfo[],
  event: z.infer<typeof SessionEventSchema>,
): SessionInfo[] {
  if (event.type === "snapshot") return [...event.sessions].sort(compare);
  // Forwards live in forwards.svelte.ts; the session list is unchanged.
  if (event.type === "forwards") return sessions;
  const id = event.type === "removed" ? event.id : event.session.id;
  const index = sessions.findIndex((session) => session.id === id);
  if (event.type === "removed") {
    if (index === -1) return sessions;
    return sessions.filter((session) => session.id !== id);
  }
  const next = [...sessions];
  if (index !== -1) {
    if (sessions[index]!.createdAt === event.session.createdAt) {
      next[index] = event.session;
      return next;
    }
    next.splice(index, 1);
  }
  const position = next.findIndex(
    (session) => compare(event.session, session) < 0,
  );
  next.splice(position === -1 ? next.length : position, 0, event.session);
  return next;
}

/**
 * The repositories and worktrees that live sessions name but the repository
 * list does not contain yet. A session started elsewhere (`webmux run
 * --new-worktree`, another browser) creates its worktree after this page
 * read the list, and `/ws/sessions` carries no workspace events, so the
 * sidebar would show it under "已不可用工作区" until something refreshed.
 */
export function unknownWorkspaces(
  sessions: readonly SessionInfo[],
  repositories: readonly Repository[],
): string[] {
  const unknown = new Set<string>();
  for (const { workspace } of sessions) {
    if (!workspace) continue;
    const repository = repositories.find(
      (r) => r.id === workspace.repositoryId,
    );
    if (!repository) unknown.add(`r:${workspace.repositoryId}`);
    else if (!repository.worktrees.some((t) => t.id === workspace.worktreeId))
      unknown.add(`w:${workspace.worktreeId}`);
  }
  return [...unknown];
}
