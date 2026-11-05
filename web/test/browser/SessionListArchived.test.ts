import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import SessionList from "../../src/pages/SessionList.svelte";
import { createTerminalDrafts } from "../../src/terminal-draft.svelte";
import type { HybridRecord, Repository, SessionInfo } from "../../src/schemas";

const record: HybridRecord = {
  id: "hybrid22",
  name: "修复排序",
  title: "修复排序",
  configId: "cfgomp01",
  configMissing: false,
  agent: "omp",
  cwd: "/work",
  cmd: ["omp"],
  side: "tui",
  agentSessionId: "01a",
  previousIds: [],
  state: "closed",
  queuedCount: 0,
  createdAt: "2026-10-28T00:00:00Z",
  updatedAt: "2026-10-28T00:00:00Z",
  interruptedAt: null,
};
const active: SessionInfo = {
  id: "hybrid33",
  generation: "0123456789abcdef",
  title: "进行中",
  cwd: "/work",
  cmd: ["omp"],
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
  hybrid: {
    side: "acp",
    state: "active",
    agent: "omp",
    agentTitle: "omp",
    agentIcon: "pi",
    agentSessionId: "01b",
  },
};

const worktree = {
  id: "main",
  repositoryId: "repo-1",
  path: "/repos/project",
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
} satisfies Repository["worktrees"][number];
const repository = {
  id: "repo-1",
  name: "project",
  commonDir: "/repos/project/.git",
  anchorPath: "/repos/project",
  availability: "available",
  error: "",
  checkedAt: "2026-10-28T00:00:00Z",
  worktrees: [
    worktree,
    {
      ...worktree,
      id: "feature",
      path: "/repos/project-feature",
      kind: "linked",
      branch: "feature",
    },
  ],
} satisfies Repository;

let records: HybridRecord[] = [];
let repositories: Repository[] = [];
const sockets: FakeSocket[] = [];

class FakeSocket {
  static readonly OPEN = 1;
  readyState = 0;
  bufferedAmount = 0;
  binaryType = "blob";
  onopen?: () => void;
  onmessage?: (event: { data: string }) => void;
  onclose?: (event: { code: number }) => void;
  onerror?: () => void;
  constructor(readonly url: string) {
    sockets.push(this);
  }
  send() {}
  close() {}
  open() {
    this.readyState = 1;
    this.onopen?.();
  }
  message(event: unknown) {
    this.onmessage?.({ data: JSON.stringify(event) });
  }
}

beforeEach(() => {
  records = [record];
  repositories = [];
  sockets.length = 0;
  vi.stubGlobal("WebSocket", FakeSocket);
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const url = new URL(String(input), location.href);
      const method = init?.method ?? "GET";
      if (url.pathname === "/api/hybrid-sessions" && method === "GET")
        return Response.json(records);
      if (url.pathname === "/api/repositories" && method === "GET")
        return Response.json(repositories);
      if (
        url.pathname === "/api/hybrid-sessions/hybrid22" &&
        method === "DELETE"
      ) {
        records = [];
        return new Response(null, { status: 204 });
      }
      // The rest of the home page degrades on its own; not under test.
      return Response.json({ error: "not found" }, { status: 404 });
    }),
  );
});

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    vi.unstubAllGlobals();
    history.replaceState(null, "", "/");
    vi.restoreAllMocks();
  }
});

function draftsWith(...ids: string[]) {
  const drafts = createTerminalDrafts();
  const entries = ids.map((id) => {
    const entry = drafts.get(id);
    drafts.setEnabled(entry, true);
    drafts.edit(entry, `draft of ${id}`);
    return entry;
  });
  return { drafts, entries };
}

async function connected() {
  await vi.waitFor(() => expect(sockets.length).toBeGreaterThan(0));
  const socket = sockets.find((s) => s.url.endsWith("/ws/sessions"))!;
  socket.open();
  return socket;
}

test("a snapshot keeps drafts of archived hybrid sessions and drops the rest", async () => {
  const {
    drafts,
    entries: [recoverable, gone],
  } = draftsWith("hybrid22", "gone0000");
  render(SessionList, { drafts });
  const socket = await connected();
  socket.message({ type: "snapshot", sessions: [], forwards: [] });
  await vi.waitFor(() => expect(drafts.valid(gone!)).toBe(false));
  expect(drafts.valid(recoverable!)).toBe(true);
  await expect
    .element(page.getByRole("region", { name: "work 的已归档会话" }))
    .toMatchTextContent(/已归档\s*1/);
});

test("archiving a hybrid session keeps its draft; deleting the record deletes it", async () => {
  records = [];
  const {
    drafts,
    entries: [closed],
  } = draftsWith("hybrid33");
  render(SessionList, { drafts });
  const socket = await connected();
  socket.message({ type: "snapshot", sessions: [active], forwards: [] });
  records = [{ ...record, id: "hybrid33", name: "进行中", title: "进行中" }];
  socket.message({ type: "removed", id: "hybrid33" });
  const group = page.getByRole("region", { name: "work 的已归档会话" });
  await expect.element(group).toMatchTextContent(/已归档\s*1/);
  expect(drafts.valid(closed!)).toBe(true);

  await group.getByRole("button", { name: /已归档/ }).click();
  await group.getByRole("button", { name: "更多操作 进行中" }).click();
  await page.getByRole("menuitem", { name: "删除记录" }).click();
  // The fetch stub only knows how to delete hybrid22; answer for this one.
  vi.mocked(fetch).mockImplementationOnce(async () => {
    records = [];
    return new Response(null, { status: 204 });
  });
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "确认" })
    .click();
  await vi.waitFor(() => expect(drafts.valid(closed!)).toBe(false));
});

const archived = (id: string, patch: Partial<HybridRecord>): HybridRecord => ({
  ...record,
  id,
  name: id,
  title: id,
  ...patch,
});

test("archived records sit in the group a live session of theirs would", async () => {
  repositories = [repository];
  records = [
    archived("recorded", {
      cwd: "/repos/project-feature",
      workspace: { repositoryId: "repo-1", worktreeId: "feature" },
    }),
    archived("legacysb", { cwd: "/repos/project/web" }),
    archived("goneworkt", {
      cwd: "/repos/project-old",
      workspace: { repositoryId: "repo-1", worktreeId: "old" },
    }),
    archived("elsewher", { cwd: "/srv/elsewhere" }),
  ];
  render(SessionList, { drafts: createTerminalDrafts() });
  const socket = await connected();
  socket.message({ type: "snapshot", sessions: [], forwards: [] });
  for (const [group, title] of [
    ["feature", "recorded"],
    ["main", "legacysb"],
    ["已不可用工作区", "goneworkt"],
    ["elsewhere", "elsewher"],
  ] as const) {
    const region = page.getByRole("region", { name: `${group} 的已归档会话` });
    await expect.element(region).toMatchTextContent(/已归档\s*1/);
    await region.getByRole("button", { name: /已归档/ }).click();
    await expect
      .element(region.getByText(title, { exact: true }))
      .toBeVisible();
  }
});

test("status and search filters cover archived records", async () => {
  records = [archived("archivd1", { title: "整理灯塔文档" })];
  render(SessionList, { drafts: createTerminalDrafts() });
  const socket = await connected();
  socket.message({
    type: "snapshot",
    sessions: [{ ...active, id: "livesess", hybrid: undefined }],
    forwards: [],
  });
  const region = page.getByRole("region", { name: "work 的已归档会话" });
  await expect.element(region).toMatchTextContent(/已归档\s*1/);
  const filters = page.getByRole("radiogroup", { name: "按会话状态筛选" });

  await filters.getByRole("radio", { name: /运行中/ }).click();
  await expect.element(region).not.toBeInTheDocument();
  await filters.getByRole("radio", { name: /已归档/ }).click();
  // Only archived records remain, revealed without a click.
  await expect.element(region.getByText("整理灯塔文档")).toBeVisible();
  await expect
    .element(page.getByText("进行中", { exact: true }))
    .not.toBeVisible();

  await filters.getByRole("radio", { name: /全部/ }).click();
  await page.getByRole("searchbox", { name: "搜索会话" }).fill("灯塔");
  await expect.element(region.getByText("整理灯塔文档")).toBeVisible();
  await page.getByRole("searchbox", { name: "搜索会话" }).fill("没有这个");
  await expect.element(region).not.toBeInTheDocument();
  // Clearing the filters brings the section back collapsed, as before.
  await page.getByRole("searchbox", { name: "搜索会话" }).click();
  await userEvent.keyboard("{Escape}");
  await expect
    .element(region.getByRole("button", { name: /已归档/ }))
    .toHaveAttribute("aria-expanded", "false");
});

test("a scope without archived matches shows the empty state", async () => {
  const other = {
    ...repository,
    id: "repo-2",
    name: "other",
    commonDir: "/repos/other/.git",
    anchorPath: "/repos/other",
    worktrees: [
      {
        ...worktree,
        repositoryId: "repo-2",
        path: "/repos/other",
      },
    ],
  } satisfies Repository;
  repositories = [repository, other];
  records = [
    archived("archivd1", {
      workspace: { repositoryId: "repo-1", worktreeId: "main" },
    }),
  ];
  render(SessionList, { drafts: createTerminalDrafts() });
  const socket = await connected();
  socket.message({ type: "snapshot", sessions: [], forwards: [] });
  await page.getByRole("button", { name: "仓库 other" }).click();
  await page
    .getByRole("radiogroup", { name: "按会话状态筛选" })
    .getByRole("radio", { name: /已归档/ })
    .click();
  await expect.element(page.getByText("没有匹配的会话")).toBeVisible();
});

test("returning home after archiving leaves the archived section collapsed", async () => {
  history.replaceState(null, "", "/");
  render(SessionList, { drafts: createTerminalDrafts() });
  const socket = await connected();
  socket.message({ type: "snapshot", sessions: [], forwards: [] });
  const region = page.getByRole("region", { name: "work 的已归档会话" });
  await expect.element(region.getByText("修复排序")).not.toBeVisible();
  expect(page.getByText("刚归档").elements()).toEqual([]);
});
