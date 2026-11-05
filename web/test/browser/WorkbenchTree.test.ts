import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import WorkbenchTree from "../../src/components/WorkbenchTree.svelte";
import type { SessionInfo } from "../../src/schemas";
import { chat, repository, session, worktree } from "./shell-fixture";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  localStorage.removeItem("webmux.workbenchTree");
});

const webmux = repository({
  worktrees: [
    worktree(),
    worktree({
      id: "otter",
      kind: "linked",
      branch: "wt/brisk-otter",
      path: "/repos/webmux.worktrees/brisk-otter",
    }),
  ],
});
const legacy = repository({
  id: "repo2",
  name: "legacy-api",
  availability: "unavailable",
  error: "路径不可用：/srv/legacy-api 不存在",
  worktrees: [],
});
const inMain = {
  repositoryId: "repo1",
  worktreeId: "main",
  worktreePath: "/repos/webmux",
};
const inOtter = {
  repositoryId: "repo1",
  worktreeId: "otter",
  worktreePath: "/repos/webmux.worktrees/brisk-otter",
};

function renderTree(
  sessions: SessionInfo[],
  props: Partial<{ current: string; scope: string; canCreate: boolean }> = {},
) {
  const handlers = {
    onscope: vi.fn(),
    onopen: vi.fn(),
    onnewworktree: vi.fn(),
    onrename: vi.fn(),
  };
  render(WorkbenchTree, {
    repositories: [webmux, legacy],
    sessions,
    // Both worktrees are on the home page.
    home: { repo1: ["main", "otter"] },
    scope: props.scope ?? "",
    current: props.current ?? "",
    canCreate: props.canCreate ?? true,
    configIcons: new Map(),
    ...handlers,
  });
  return handlers;
}

const tree = () => page.getByRole("tree", { name: "仓库与会话" });
// Scope rows add their counts after a comma; session rows their state.
const item = (name: string) =>
  tree().getByRole("treeitem", {
    name: new RegExp(`^${name.replace(/[()（）]/g, "\\$&")}(，|$)`),
  });

test("repositories hold the home page's worktrees and their sessions", async () => {
  renderTree(
    [
      chat({
        id: "agent001",
        title: "修复排序",
        workspace: inMain,
        createdAt: "2026-10-28T00:00:00Z",
      }),
      session({
        id: "shell001",
        title: "pnpm dev",
        workspace: inMain,
        createdAt: "2026-10-28T01:00:00Z",
      }),
      session({
        id: "ended001",
        title: "make test",
        status: "exited",
        exitCode: 0,
        workspace: inOtter,
      }),
      session({ id: "direct01", title: "htop" }),
    ],
    { current: "ended001" },
  );
  // Level one: all, each repository, direct sessions; in that order.
  const top = tree()
    .getByRole("treeitem")
    .elements()
    .filter((el) => el.getAttribute("aria-level") === "1");
  expect(top.map((el) => el.textContent?.trim().split(/\s/)[0])).toEqual([
    "全部仓库",
    "webmux",
    "legacy-api",
    "直接会话",
  ]);
  // The main worktree is folded by default; the current session's path is
  // open, and the newer session comes first.
  await expect
    .element(item("make test（已结束）"))
    .toHaveAttribute("aria-current", "page");
  await expect.element(item("make test（已结束）")).toBeVisible();
  await expect.element(item("main")).toHaveAttribute("aria-expanded", "false");
  await tree().getByRole("button", { name: "展开main" }).click();
  await tree().getByRole("button", { name: "展开直接会话" }).click();
  const rows = tree()
    .getByRole("treeitem", { name: /（(空闲|工作中|等待|已结束)）$/ })
    .elements();
  expect(rows.map((row) => row.getAttribute("aria-label"))).toEqual([
    "pnpm dev（空闲）",
    "修复排序（空闲）",
    "make test（已结束）",
    "htop（空闲）",
  ]);
  // The unavailable repository stays, marked, without worktrees or "+".
  await expect.element(item("legacy-api（不可用）")).toBeVisible();
  expect(item("legacy-api（不可用）").element().getAttribute("title")).toBe(
    "legacy-api：路径不可用：/srv/legacy-api 不存在",
  );
  expect(
    tree().getByRole("button", { name: "在 legacy-api 新建工作树" }).elements(),
  ).toEqual([]);
});

test("rows navigate: scopes, worktrees and sessions", async () => {
  const live = session({
    id: "shell001",
    title: "pnpm dev",
    workspace: inMain,
  });
  const handlers = renderTree([live], { scope: "all" });
  await expect
    .element(item("全部仓库"))
    .toHaveAttribute("aria-current", "page");
  await item("webmux").click();
  expect(handlers.onscope).toHaveBeenLastCalledWith("repo1");
  await item("直接会话").click();
  expect(handlers.onscope).toHaveBeenLastCalledWith("direct");
  await item("wt/brisk-otter").click();
  expect(handlers.onscope).toHaveBeenLastCalledWith("repo1", "otter");
  await tree().getByRole("button", { name: "展开main" }).click();
  await item("pnpm dev（空闲）").click();
  expect(handlers.onopen).toHaveBeenCalledWith(live);
});

test("each available repository offers the new-worktree flow", async () => {
  const handlers = renderTree([]);
  await tree().getByRole("button", { name: "在 webmux 新建工作树" }).click();
  expect(handlers.onnewworktree).toHaveBeenCalledWith("repo1");
  await cleanup();
  renderTree([], { canCreate: false });
  await expect.element(item("webmux")).toBeVisible();
  expect(page.getByRole("button", { name: /新建工作树/ }).elements()).toEqual(
    [],
  );
});

test("folded nodes keep waiting counts, and folds survive a remount", async () => {
  renderTree([
    chat({
      id: "agent001",
      title: "等权限",
      workspace: inOtter,
      status: "waiting",
      attention: {
        kind: "permission",
        source: "agent",
        since: "2026-10-28T00:00:00Z",
      },
    }),
  ]);
  await expect
    .element(item("wt/brisk-otter"))
    .toHaveAttribute("aria-expanded", "false");
  await expect
    .element(item("wt/brisk-otter"))
    .toHaveAccessibleName("wt/brisk-otter，1 个会话，1 个等待");
  await expect
    .element(item("webmux"))
    .toHaveAccessibleName("webmux，1 个会话，1 个等待");
  await tree().getByRole("button", { name: "收起webmux" }).click();
  await expect
    .element(item("webmux"))
    .toHaveAttribute("aria-expanded", "false");
  await expect
    .element(item("webmux"))
    .toHaveAccessibleName("webmux，1 个会话，1 个等待");
  await cleanup();
  renderTree([]);
  await expect
    .element(item("webmux"))
    .toHaveAttribute("aria-expanded", "false");
});

test("arrow keys move through the visible rows; the action is the next tab stop", async () => {
  renderTree([
    session({ id: "shell001", title: "pnpm dev", workspace: inMain }),
  ]);
  await item("全部仓库").click();
  await userEvent.keyboard("{ArrowDown}");
  await expect.element(item("webmux")).toHaveFocus();
  await userEvent.keyboard("{ArrowDown}");
  await expect.element(item("main")).toHaveFocus();
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(item("main")).toHaveAttribute("aria-expanded", "true");
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(item("pnpm dev（空闲）")).toHaveFocus();
  await userEvent.keyboard("{ArrowLeft}");
  await expect.element(item("main")).toHaveFocus();
  await userEvent.keyboard("{ArrowLeft}");
  await expect.element(item("main")).toHaveAttribute("aria-expanded", "false");
  await userEvent.keyboard("{ArrowLeft}");
  await expect.element(item("webmux")).toHaveFocus();
  // One stop for the rows; the focused repository's "+" comes next.
  await userEvent.tab();
  await expect
    .element(tree().getByRole("button", { name: "在 webmux 新建工作树" }))
    .toHaveFocus();
});
