// How a worktree or branch stands against its upstream, and the words the
// UI uses for it. Every count is local: ahead and behind compare with the
// remote-tracking refs from the last fetch.
import { relativeTime } from "./commit-choices";
import type {
  Branch,
  Worktree,
  WorktreeChanges,
  WorktreeOperation,
} from "./schemas";

type Tracking = Pick<Branch, "upstream" | "ahead" | "behind" | "upstreamGone">;

export type UpstreamState =
  | { kind: "none" }
  | { kind: "gone"; upstream: string }
  | { kind: "uncounted"; upstream: string }
  | { kind: "tracked"; upstream: string; ahead: number; behind: number };

/** The upstream state of a branch or worktree. */
export function upstreamState(tracking: Tracking): UpstreamState {
  const { upstream, ahead, behind } = tracking;
  if (upstream === null) return { kind: "none" };
  if (tracking.upstreamGone) return { kind: "gone", upstream };
  if (ahead === null || behind === null) return { kind: "uncounted", upstream };
  return { kind: "tracked", upstream, ahead, behind };
}

/**
 * A worktree's upstream state; `unknown` when its status was not read, and
 * null where an upstream means nothing: bare, detached or without commits.
 */
export function worktreeUpstream(
  tree: Worktree,
): UpstreamState | { kind: "unknown" } | null {
  if (tree.kind === "bare") return null;
  if (!tree.available || tree.changes === null) return { kind: "unknown" };
  if (tree.detached || tree.head === null) return null;
  return upstreamState(tree);
}

/**
 * The home title's indicator shows only what needs attention: commits
 * ahead or behind, a deleted upstream, an operation in progress.
 */
export function syncNotable(tree: Worktree): boolean {
  const state = worktreeUpstream(tree);
  return (
    tree.operation !== null ||
    state?.kind === "gone" ||
    (state?.kind === "tracked" && (state.ahead > 0 || state.behind > 0))
  );
}

/**
 * Whether a worktree's branch can be switched in place: bare, missing or
 * prunable worktrees have no checkout to switch.
 */
export function switchableWorktree(tree: Worktree): boolean {
  return tree.kind !== "bare" && tree.available && !tree.prunable;
}

/** The words of a state without counts to show. */
export function upstreamText(
  state: UpstreamState | { kind: "unknown" },
): string {
  switch (state.kind) {
    case "none":
      return "无上游";
    case "gone":
      return "上游已删除";
    case "uncounted":
      return "未计算";
    case "unknown":
      return "未知";
    case "tracked":
      return state.ahead || state.behind ? "" : "已同步";
  }
}

export const aheadLabel = (count: number) => `领先上游 ${count} 个提交`;
export const behindLabel = (count: number) => `落后上游 ${count} 个提交`;

export const operationLabels: Record<WorktreeOperation, string> = {
  merge: "正在合并",
  rebase: "正在变基",
  "cherry-pick": "正在拣选提交",
  revert: "正在还原提交",
  bisect: "正在二分查找",
};

/** Change counts in display order. */
export function changeCounts(changes: WorktreeChanges) {
  return [
    { label: "已暂存", count: changes.staged },
    { label: "未暂存", count: changes.unstaged },
    { label: "未跟踪", count: changes.untracked },
    { label: "冲突", count: changes.conflicted },
  ];
}

/** The last fetch's age (labels mark it approximate), or 未知 if never. */
export function fetchedText(lastFetchedAt: string | null, now = Date.now()) {
  return lastFetchedAt ? relativeTime(lastFetchedAt, now) : "未知";
}
