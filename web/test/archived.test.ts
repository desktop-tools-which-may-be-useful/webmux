import { describe, expect, test } from "vitest";
import {
  placeArchived,
  recordWorktree,
  removableRecordWorktree,
  repositoryRecords,
} from "../src/archived";
import type { HybridRecord, Repository, Worktree } from "../src/schemas";

const tree = (id: string, path: string): Worktree => ({
  id,
  repositoryId: "r1",
  path,
  kind: "linked",
  branch: id,
  head: null,
  detached: false,
  locked: false,
  lockReason: "",
  prunable: false,
  available: true,
  dirty: false,
  hasUntracked: false,
  hasIgnored: false,
  error: "",
});
const repository: Repository = {
  id: "r1",
  name: "project",
  commonDir: "/work/main/.git",
  anchorPath: "/work/main",
  availability: "available",
  error: "",
  checkedAt: "2026-10-20T00:00:00Z",
  initInstruction: "",
  worktrees: [tree("main", "/work/main"), tree("nested", "/work/main/nested")],
};
const record = (id: string, patch: Partial<HybridRecord>): HybridRecord => ({
  id,
  name: id,
  title: id,
  configId: "cfg",
  configMissing: false,
  agent: "omp",
  cwd: "/elsewhere",
  cmd: ["omp"],
  side: "tui",
  agentSessionId: null,
  previousIds: [],
  state: "closed",
  queuedCount: 0,
  createdAt: "2026-10-28T00:00:00Z",
  updatedAt: "2026-10-28T00:00:00Z",
  interruptedAt: null,
  ...patch,
});
const ids = (records: readonly HybridRecord[] = []) => records.map((r) => r.id);

describe("placeArchived", () => {
  const records = [
    // The recorded association wins over the directory.
    record("recorded", {
      cwd: "/work/main/nested",
      workspace: { repositoryId: "r1", worktreeId: "main" },
    }),
    record("gone", {
      cwd: "/work/gone",
      workspace: { repositoryId: "r1", worktreeId: "gone" },
    }),
    // Legacy records join the deepest worktree containing their directory.
    record("deep", { cwd: "/work/main/nested/src/" }),
    record("root", { cwd: "/work/main" }),
    // A sibling path with the same prefix is not inside the worktree.
    record("sibling", { cwd: "/work/main-copy" }),
    record("removed", {
      cwd: "/work/main",
      workspace: { repositoryId: "r9", worktreeId: "main" },
    }),
  ];

  test("places every record exactly once", () => {
    const placed = placeArchived(records, [repository]);
    const entry = placed.repositories.get("r1")!;
    expect(ids(entry.worktrees.get("main"))).toEqual(["recorded", "root"]);
    expect(ids(entry.worktrees.get("nested"))).toEqual(["deep"]);
    expect(ids(entry.missing)).toEqual(["gone"]);
    expect(ids(placed.direct)).toEqual(["sibling", "removed"]);
    expect(repositoryRecords(entry).length + placed.direct.length).toBe(
      records.length,
    );
  });

  test("without repositories, as when Git cannot be read, nothing is lost", () => {
    const placed = placeArchived(records, []);
    expect(placed.repositories.size).toBe(0);
    expect(ids(placed.direct)).toEqual(ids(records));
  });

  test("resolves removable linked worktrees for archived records", () => {
    const repos: Repository[] = [
      {
        ...repository,
        worktrees: [
          { ...tree("main", "/work/main"), kind: "main" },
          tree("nested", "/work/main/nested"),
          { ...tree("prunable", "/work/prunable"), prunable: true },
        ],
      },
    ];
    expect(recordWorktree(records[0]!, repos)?.id).toBe("main");
    expect(removableRecordWorktree(records[0]!, repos)).toBeUndefined();
    expect(removableRecordWorktree(records[1]!, repos)).toBeUndefined();
    expect(removableRecordWorktree(records[2]!, repos)?.id).toBe("nested");
    expect(
      removableRecordWorktree(
        record("stale", {
          cwd: "/work/prunable",
          workspace: { repositoryId: "r1", worktreeId: "prunable" },
        }),
        repos,
      ),
    ).toBeUndefined();
  });
});
