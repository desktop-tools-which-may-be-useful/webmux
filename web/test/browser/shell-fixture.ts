import type {
  HybridRecord,
  LaunchConfigSummary,
  Repository,
  SessionInfo,
  Worktree,
} from "../../src/schemas";

// Data for the app shell's tests: sessions, repositories, archived records
// and launch configs as the server sends them.

export function session(patch: Partial<SessionInfo> = {}): SessionInfo {
  return {
    id: "shellzzz",
    generation: "0123456789abcdef",
    title: "bash",
    cwd: "/work",
    cmd: ["bash"],
    status: "idle",
    exitCode: null,
    listeningPorts: [],
    preview: "",
    createdAt: "2026-10-28T00:00:00Z",
    lastActiveAt: "2026-10-28T00:00:00Z",
    cols: 80,
    rows: 24,
    clients: 0,
    workspace: null,
    attention: null,
    reportedCwd: null,
    ...patch,
  };
}

export function chat(
  patch: Partial<SessionInfo> = {},
  side: "acp" | "tui" = "acp",
): SessionInfo {
  return session({
    cmd: ["claude"],
    hybrid: {
      side,
      state: "active",
      agent: "claude",
      agentTitle: "Claude Code",
      agentIcon: "claudecode",
      agentSessionId: "01a",
    },
    ...patch,
  });
}

export function worktree(patch: Partial<Worktree> = {}): Worktree {
  return {
    id: "main",
    repositoryId: "repo1",
    path: "/repos/webmux",
    kind: "main",
    branch: "main",
    head: "a".repeat(40),
    detached: false,
    locked: false,
    lockReason: "",
    prunable: false,
    available: true,
    dirty: false,
    hasUntracked: false,
    hasIgnored: false,
    error: "",
    ...patch,
  };
}

export function repository(patch: Partial<Repository> = {}): Repository {
  return {
    id: "repo1",
    name: "webmux",
    commonDir: "/repos/webmux/.git",
    anchorPath: "/repos/webmux",
    availability: "available",
    error: "",
    checkedAt: "2026-10-28T00:00:00Z",
    initInstruction: "",
    worktrees: [worktree()],
    ...patch,
  };
}

export function record(patch: Partial<HybridRecord> = {}): HybridRecord {
  return {
    id: "archivea",
    name: "",
    title: "整理文档",
    configId: "cfgclaud",
    configMissing: false,
    agent: "claude",
    cwd: "/repos/webmux",
    cmd: ["claude"],
    side: "acp",
    agentSessionId: "01c",
    previousIds: [],
    state: "closed",
    queuedCount: 0,
    createdAt: "2026-10-27T00:00:00Z",
    updatedAt: "2026-10-27T00:00:00Z",
    interruptedAt: null,
    ...patch,
  };
}

export function launchConfig(
  patch: Partial<LaunchConfigSummary> = {},
): LaunchConfigSummary {
  return {
    id: "cfgclaud",
    name: "Claude Code",
    icon: "claudecode",
    cmd: ["claude"],
    envNames: [],
    chatOptions: [],
    preview: null,
    hybrid: true,
    hybridSupport: { supported: true, supportedAgents: [], agent: "claude" },
    createdAt: "2026-10-01T00:00:00Z",
    updatedAt: "2026-10-01T00:00:00Z",
    ...patch,
  };
}
