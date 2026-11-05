import type { ResumeChoice, ResumeOutcome } from "./recoverable";
import type {
  HybridRecord,
  Repository,
  SessionInfo,
  Worktree,
} from "./schemas";

// Archived records are the closed and interrupted hybrid sessions. The home
// page shows each one under the group a live session would have: its
// worktree, its repository's unavailable-workspace group, or the direct
// session group of its directory.

export interface RepositoryArchive {
  /** Records by worktree ID, for worktrees in the repository's Git list. */
  worktrees: ReadonlyMap<string, HybridRecord[]>;
  /** Records whose worktree is no longer in the Git list. */
  missing: HybridRecord[];
}

export interface ArchivePlacement {
  repositories: ReadonlyMap<string, RepositoryArchive>;
  /** Records outside every registered repository, grouped by `cwd` later. */
  direct: HybridRecord[];
}

/** What every archived section needs from the home page. */
export interface ArchiveActions {
  now: number;
  /** Filters are active: sections with matches open on their own. */
  filtering: boolean;
  resume: (
    id: string,
    side: "tui" | "acp",
    choice?: ResumeChoice,
  ) => Promise<ResumeOutcome>;
  remove: (id: string) => Promise<unknown>;
  /** Opens the worktree removal confirmation when the record's linked worktree still exists. */
  removeWorktree?: (worktree: Worktree) => void;
  onresumed: (session: SessionInfo) => void;
}

export const EMPTY_ARCHIVE: RepositoryArchive = {
  worktrees: new Map(),
  missing: [],
};

function trimmed(path: string) {
  return path.length > 1 ? path.replace(/\/+$/, "") : path;
}

function within(root: string, path: string) {
  const base = trimmed(root);
  const target = trimmed(path);
  return target === base || target.startsWith(base === "/" ? "/" : `${base}/`);
}

/**
 * Resolves the registered worktree a record belongs to: its recorded
 * worktree association wins; a record without one (written before
 * associations were recorded) joins the registered worktree with the
 * longest path containing its `cwd`.
 */
export function recordWorktree(
  record: HybridRecord,
  repositories: readonly Repository[],
): Worktree | undefined {
  const recorded = record.workspace;
  if (recorded) {
    const repository = repositories.find(
      (repo) => repo.id === recorded.repositoryId,
    );
    return repository?.worktrees.find(
      (tree) => tree.id === recorded.worktreeId,
    );
  }
  let best: { tree: Worktree; length: number } | undefined;
  for (const repo of repositories)
    for (const tree of repo.worktrees)
      if (
        within(tree.path, record.cwd) &&
        (!best || trimmed(tree.path).length > best.length)
      )
        best = { tree, length: trimmed(tree.path).length };
  return best?.tree;
}

/**
 * The linked worktree that deleting `record` should remove through the
 * worktree confirmation dialog, or undefined when only the record itself can
 * be deleted (main/bare checkout, unavailable directory, or worktree already
 * gone).
 */
export function removableRecordWorktree(
  record: HybridRecord,
  repositories: readonly Repository[],
): Worktree | undefined {
  const tree = recordWorktree(record, repositories);
  return tree && tree.kind === "linked" && tree.available && !tree.prunable
    ? tree
    : undefined;
}

/**
 * Places every record exactly once: a recorded worktree association wins;
 * a record without one (written before associations were recorded) joins
 * the registered worktree with the longest path containing its `cwd`; any
 * other record is a direct one. Record order is kept, newest first.
 */
export function placeArchived(
  records: readonly HybridRecord[],
  repositories: readonly Repository[],
): ArchivePlacement {
  const placed = new Map<
    string,
    { worktrees: Map<string, HybridRecord[]>; missing: HybridRecord[] }
  >();
  const direct: HybridRecord[] = [];
  const repositoryOf = (id: string) => {
    let entry = placed.get(id);
    if (!entry) {
      entry = { worktrees: new Map(), missing: [] };
      placed.set(id, entry);
    }
    return entry;
  };
  const toWorktree = (
    repositoryId: string,
    worktreeId: string,
    record: HybridRecord,
  ) => {
    const entry = repositoryOf(repositoryId);
    const list = entry.worktrees.get(worktreeId);
    if (list) list.push(record);
    else entry.worktrees.set(worktreeId, [record]);
  };
  for (const record of records) {
    const tree = recordWorktree(record, repositories);
    if (tree) {
      toWorktree(tree.repositoryId, tree.id, record);
      continue;
    }
    const recorded = record.workspace;
    if (
      recorded &&
      repositories.some((repo) => repo.id === recorded.repositoryId)
    ) {
      repositoryOf(recorded.repositoryId).missing.push(record);
      continue;
    }
    direct.push(record);
  }
  return { repositories: placed, direct };
}

/** Every record placed in one repository, worktree by worktree. */
export function repositoryRecords(archive: RepositoryArchive) {
  return [...archive.worktrees.values(), archive.missing].flat();
}

export function matchesArchivedQuery(
  record: HybridRecord,
  search: string,
  repositoryName?: string,
  branch?: string | null,
) {
  return [
    record.title,
    record.name,
    record.agent,
    record.cwd,
    repositoryName,
    branch,
  ].some((value) => value?.toLocaleLowerCase().includes(search));
}
