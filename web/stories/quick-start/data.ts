import type { SessionInfo } from "../../src/schemas";
import { session as base } from "../fixtures";
import { titleOf } from "./titles.svelte";

// Sample data shared by the prototype's chat history, workbench tree and
// workbench overview: one set of sessions seen from both modes.
export type Mode = "chat" | "workbench";
export type ChatStatus = "working" | "waiting" | "idle" | "ended";
export type Entry = {
  id: string;
  title: string;
  icon: string;
  agentTitle: string;
  status: ChatStatus;
  /** The hybrid session's side; null for an ordinary terminal session. */
  side: "acp" | "tui" | null;
};
export type Tree = {
  id: string;
  title: string;
  path: string;
  entries: Entry[];
};
export type Repo = {
  id: string;
  name: string;
  reason?: string;
  worktrees: Tree[];
};

export const statusLabels: Record<ChatStatus, string> = {
  working: "工作中",
  waiting: "等待",
  idle: "空闲",
  ended: "已结束",
};

const entry = (
  id: string,
  title: string,
  icon: string,
  agentTitle: string,
  status: ChatStatus,
  side: Entry["side"] = "acp",
): Entry => ({ id, title, icon, agentTitle, status, side });

export const repositories: Repo[] = [
  {
    id: "webmux",
    name: "webmux",
    worktrees: [
      {
        id: "wt-main",
        title: "main",
        path: "/home/developer/webmux",
        entries: [
          entry("s3", "Markdown 文件行长校验", "opencode", "OpenCode", "idle"),
          entry("t1", "Shell · webmux", "terminal", "Shell", "idle", null),
          // A hybrid session switched to its terminal side.
          entry(
            "s4",
            "快照回放回调问题",
            "claudecode",
            "Claude Code",
            "idle",
            "tui",
          ),
        ],
      },
      {
        id: "wt-brisk",
        title: "brisk-otter",
        path: "/home/developer/webmux.worktrees/brisk-otter",
        entries: [
          entry(
            "s1",
            "修复首页置顶会话排序",
            "claudecode",
            "Claude Code",
            "working",
          ),
          entry("t2", "pnpm dev", "terminal", "Shell", "working", null),
        ],
      },
    ],
  },
  {
    id: "docs-site",
    name: "docs-site",
    worktrees: [
      {
        id: "wt-docs",
        title: "main",
        path: "/home/developer/docs-site",
        entries: [
          entry("s2", "项目文档治理", "pi", "omp", "waiting"),
          entry("t3", "make test", "terminal", "Shell", "ended", null),
        ],
      },
    ],
  },
  {
    id: "legacy-api",
    name: "legacy-api",
    reason: "路径不可用：/srv/legacy-api 不存在",
    worktrees: [],
  },
];

export const directEntries: Entry[] = [
  entry("d1", "htop", "terminal", "Shell", "idle", null),
];

// Chat history: the live hybrid sessions above plus archived records,
// newest activity first.
const live = [
  ...repositories.flatMap((repo) => repo.worktrees),
  { entries: directEntries },
].flatMap((tree) => tree.entries);

export const chats: Entry[] = [
  ...["s1", "s2", "s3", "s4"].map((id) => live.find((item) => item.id === id)!),
  entry(
    "a1",
    "终端缓冲区调整大小性能：减少重排时的整屏重绘并保留滚动位置",
    "codex",
    "Codex",
    "ended",
  ),
  entry("a2", "CI 配置优化", "pi", "omp", "ended"),
  entry("a3", "AGENTS.md 文件大小优化", "claudecode", "Claude Code", "ended"),
  entry("a4", "移动端加载闪屏问题", "opencode", "OpenCode", "ended"),
];

/** A live session by ID; archived chats are not live. */
export const liveEntry = (id: string) => live.find((item) => item.id === id);

const states: Record<ChatStatus, SessionInfo["status"]> = {
  working: "running",
  waiting: "waiting",
  idle: "idle",
  ended: "exited",
};

/** A session card for the workbench overview, built on the story fixture. */
export function sessionInfo(item: Entry, cwd: string): SessionInfo {
  return {
    ...base,
    id: item.id,
    title: titleOf(item),
    cwd,
    status: states[item.status],
    exitCode: item.status === "ended" ? 0 : null,
    clients: item.status === "working" ? 1 : 0,
    preview: item.side
      ? `› ${item.title}\n\n  ${item.agentTitle} · ${statusLabels[item.status]}`
      : `➜  ${item.title}`,
    reportedCwd: null,
  };
}
