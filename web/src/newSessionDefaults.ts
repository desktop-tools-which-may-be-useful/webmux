import type { LaunchConfigsState } from "./launchConfigs";
import type { Repository, SessionInfo, Worktree } from "./schemas";

/** The contextual entry the new-session dialog was opened from, if any. */
export type LaunchEntry =
  | { kind: "worktree"; repositoryId: string; worktreeId: string }
  | { kind: "new-worktree"; repositoryId: string }
  | { kind: "directory"; cwd: string; workspace: SessionInfo["workspace"] };

/**
 * The dialog's location. A worktree target with an empty `worktreeId` is the
 * unset state: the repository is kept but the user must pick a target.
 */
export type TargetKind = "directory" | "worktree" | "new-worktree";
export type InitialTarget = {
  kind: TargetKind;
  cwd: string;
  repositoryId: string;
  worktreeId: string;
};

export function selectableWorktree(tree: Worktree) {
  return tree.available && tree.kind !== "bare";
}

export function worktreeLabel(tree: Worktree) {
  return `${tree.kind === "main" ? "主工作区 · " : ""}${tree.branch || tree.head?.slice(0, 12) || "无提交"}`;
}

/**
 * Initial location by precedence: an explicit contextual entry, then the home
 * page's selected repository (its available main worktree, else another
 * available non-bare tree, else unset), then the latest recent directory,
 * then the service default.
 */
export function resolveInitialTarget(input: {
  entry?: LaunchEntry;
  selectedRepositoryId?: string;
  repositories: readonly Repository[];
  recent: readonly string[];
  defaultCwd: string;
}): InitialTarget {
  const fallbackCwd = input.recent[0] || input.defaultCwd;
  const { entry } = input;
  if (entry?.kind === "directory")
    return {
      kind: "directory",
      cwd: entry.cwd || fallbackCwd,
      repositoryId: entry.workspace?.repositoryId ?? "",
      worktreeId: "",
    };
  if (entry)
    return {
      kind: entry.kind,
      cwd: fallbackCwd,
      repositoryId: entry.repositoryId,
      worktreeId: entry.kind === "worktree" ? entry.worktreeId : "",
    };
  const repository = input.repositories.find(
    (repo) => repo.id === input.selectedRepositoryId,
  );
  if (!repository)
    return {
      kind: "directory",
      cwd: fallbackCwd,
      repositoryId: "",
      worktreeId: "",
    };
  const trees = repository.worktrees;
  const tree =
    trees.find((item) => item.kind === "main" && selectableWorktree(item)) ??
    trees.find(selectableWorktree);
  return {
    kind: "worktree",
    cwd: fallbackCwd,
    repositoryId: repository.id,
    worktreeId: tree?.id ?? "",
  };
}

export type LaunchSource = "config" | "custom" | "unset";
export type InitialLaunch = { source: LaunchSource; configId: string };

/**
 * Initial command from the remembered `lastLaunch` (`config:<ID>` or
 * `custom`). A remembered config that no longer exists is never replaced by
 * another one; without a memory the first config is used, else a custom
 * command. Returns null while the remembered or default config is loading.
 */
export function resolveInitialLaunch(
  lastLaunch: string,
  launchConfigs: LaunchConfigsState,
): InitialLaunch | null {
  if (lastLaunch === "custom") return { source: "custom", configId: "" };
  const remembered =
    lastLaunch.startsWith("config:") && lastLaunch.length > 7
      ? lastLaunch.slice(7)
      : "";
  if (launchConfigs.status === "loading") return null;
  const configs = launchConfigs.status === "ready" ? launchConfigs.configs : [];
  if (remembered)
    return configs.some((config) => config.id === remembered)
      ? { source: "config", configId: remembered }
      : { source: "unset", configId: "" };
  const first = configs[0]?.id;
  return first
    ? { source: "config", configId: first }
    : { source: "custom", configId: "" };
}

/** The last path segment, used as a short directory name. */
export function directoryName(path: string) {
  return path.replace(/\/+$/, "").split("/").pop() || "/";
}
