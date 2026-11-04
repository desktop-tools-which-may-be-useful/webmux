import type { ResumeChoice, ResumeOutcome } from "./recoverable";
import type { HybridRecord, Repository, SessionInfo } from "./schemas";

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
    const recorded = record.workspace;
    const repository =
      recorded &&
      repositories.find((repo) => repo.id === recorded.repositoryId);
    if (recorded && repository) {
      if (repository.worktrees.some((tree) => tree.id === recorded.worktreeId))
        toWorktree(repository.id, recorded.worktreeId, record);
      else repositoryOf(repository.id).missing.push(record);
      continue;
    }
    if (!recorded) {
      let best:
        | { repositoryId: string; worktreeId: string; length: number }
        | undefined;
      for (const repo of repositories)
        for (const tree of repo.worktrees)
          if (
            within(tree.path, record.cwd) &&
            (!best || trimmed(tree.path).length > best.length)
          )
            best = {
              repositoryId: repo.id,
              worktreeId: tree.id,
              length: trimmed(tree.path).length,
            };
      if (best) {
        toWorktree(best.repositoryId, best.worktreeId, record);
        continue;
      }
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
