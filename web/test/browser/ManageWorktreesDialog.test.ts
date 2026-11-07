import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import HomeWorktreeDialog from "../../src/components/HomeWorktreeDialog.svelte";
import ManageWorktreesDialog from "../../src/components/ManageWorktreesDialog.svelte";
import type { PushCheck } from "../../src/schemas";
import { chat, record, repository, worktree } from "./shell-fixture";

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    vi.unstubAllGlobals();
  }
});

const mainTree = worktree({
  id: "main",
  repositoryId: "repo1",
  kind: "main",
  branch: "main",
  path: "/repos/webmux",
});

const unpinnedClean = worktree({
  id: "clean1",
  repositoryId: "repo1",
  kind: "linked",
  branch: "wt/clean-feature",
  path: "/repos/webmux.worktrees/clean-feature",
  head: "a".repeat(40),
});

const prunableTree = worktree({
  id: "gone1",
  repositoryId: "repo1",
  kind: "linked",
  branch: "wt/stale-reg",
  path: "/repos/webmux.worktrees/stale-reg",
  head: "b".repeat(40),
  prunable: true,
  available: false,
  dirty: null,
});

const blockedTree = worktree({
  id: "dirty1",
  repositoryId: "repo1",
  kind: "linked",
  branch: "wt/wip-agent",
  path: "/repos/webmux.worktrees/wip-agent",
  head: "c".repeat(40),
  dirty: true,
});

const lockedTree = worktree({
  id: "lock1",
  repositoryId: "repo1",
  kind: "linked",
  branch: "wt/locked-prod",
  path: "/repos/webmux.worktrees/locked-prod",
  locked: true,
  lockReason: "prod",
});

const repo = repository({
  id: "repo1",
  name: "webmux",
  worktrees: [mainTree, unpinnedClean, prunableTree, blockedTree, lockedTree],
});

const liveChat = chat({
  id: "livechat",
  title: "实现批量清理",
  cwd: blockedTree.path,
  workspace: {
    repositoryId: "repo1",
    worktreeId: blockedTree.id,
    worktreePath: blockedTree.path,
  },
});

const keptArchive = record({
  id: "keptabcd",
  title: "归档讨论",
  cwd: prunableTree.path,
});

const noRemote: PushCheck = { state: "noRemote", branches: [], more: 0 };

test("lists all worktrees including unpinned ones, filters, selects safe items, and executes safe-only batch removal by default", async () => {
  const fetch = vi.fn<typeof globalThis.fetch>(async (input) => {
    const url = String(input);
    if (url.endsWith("/batch-removal-preview")) {
      return Response.json({
        confirmationId: "batchconf1",
        expiresAt: "2099-01-01T00:00:00Z",
        items: [
          {
            worktree: unpinnedClean,
            sessions: [],
            archived: [],
            commit: "a".repeat(40),
            push: noRemote,
            blockers: [],
            warnings: [],
            forceAllowed: true,
          },
          {
            worktree: prunableTree,
            sessions: [],
            archived: [keptArchive],
            commit: "b".repeat(40),
            push: { state: "unpushed", branches: [], more: 0 },
            blockers: [],
            warnings: [],
            forceAllowed: true,
          },
          {
            worktree: blockedTree,
            sessions: [liveChat],
            archived: [],
            commit: "c".repeat(40),
            push: noRemote,
            blockers: ["存在未提交修改", "存在运行中的 WebMux 会话"],
            warnings: [],
            forceAllowed: true,
          },
        ],
      });
    }
    return Response.json({
      results: [
        {
          worktreeId: unpinnedClean.id,
          path: unpinnedClean.path,
          branch: "wt/clean-feature",
          status: "removed",
          error: "",
        },
        {
          worktreeId: prunableTree.id,
          path: prunableTree.path,
          branch: "wt/stale-reg",
          status: "removed",
          error: "",
        },
        {
          worktreeId: blockedTree.id,
          path: blockedTree.path,
          branch: "wt/wip-agent",
          status: "skipped",
          error: "",
        },
      ],
    });
  });
  vi.stubGlobal("fetch", fetch);

  const onchange = vi.fn();
  const onremoved = vi.fn();
  const onclose = vi.fn();

  render(ManageWorktreesDialog, {
    repository: repo,
    sessions: [liveChat],
    archived: [keptArchive],
    home: {}, // Only main is pinned
    onchange,
    onremoved,
    onclose,
  });

  const dialog = page.getByRole("dialog", { name: "管理工作树 · webmux" });
  await expect.element(dialog).toBeVisible();
  await expect.element(dialog).toMatchTextContent(/未添加到主页/);
  await expect.element(dialog).toMatchTextContent(/Git 登记待清理/);

  // Protected items have disabled checkboxes.
  await expect
    .element(dialog.getByRole("checkbox", { name: "main" }))
    .toBeDisabled();
  await expect
    .element(dialog.getByRole("checkbox", { name: "wt/locked-prod" }))
    .toBeDisabled();

  // Select all safe items (2), then also check the blocked item.
  await dialog.getByRole("button", { name: "选中全部可安全清理（2）" }).click();
  await dialog.getByRole("checkbox", { name: "wt/wip-agent" }).click();
  await expect
    .element(dialog.getByRole("button", { name: "下一步：预览删除 (3)" }))
    .toBeEnabled();

  await dialog.getByRole("button", { name: "下一步：预览删除 (3)" }).click();

  const previewDialog = page.getByRole("dialog", {
    name: "确认批量清理 · webmux",
  });
  await expect
    .element(previewDialog)
    .toMatchTextContent(/可安全清理 2 个.*有阻塞 1 个/);
  await expect
    .element(previewDialog)
    .toMatchTextContent(/清理失效登记.*保留 1 条归档聊天/);
  await expect
    .element(previewDialog)
    .toMatchTextContent(/默认跳过，不改动文件与会话/);

  // Default submit runs safe-only without includeForce.
  await previewDialog.getByRole("button", { name: "确认安全删除 (2)" }).click();

  const lastCall = fetch.mock.calls.at(-1);
  expect(lastCall?.[0]).toBe("/api/repositories/repo1/worktrees/batch-remove");
  expect(JSON.parse(String(lastCall?.[1]?.body))).toEqual({
    confirmationId: "batchconf1",
    includeForce: false,
    closeSessions: false,
  });
  expect(onremoved).toHaveBeenCalledWith(["clean1", "gone1"]);
  expect(onchange).toHaveBeenCalledOnce();
  expect(onclose).toHaveBeenCalledOnce();
});

test("supports explicit force authorization and keeps failed items selected for retry", async () => {
  let previewCalls = 0;
  const fetch = vi.fn<typeof globalThis.fetch>(async (input) => {
    const url = String(input);
    if (url.endsWith("/batch-removal-preview")) {
      previewCalls++;
      return Response.json({
        confirmationId: `conf-${previewCalls}`,
        expiresAt: "2099-01-01T00:00:00Z",
        items:
          previewCalls === 1
            ? [
                {
                  worktree: unpinnedClean,
                  sessions: [],
                  archived: [],
                  commit: "a".repeat(40),
                  push: noRemote,
                  blockers: [],
                  warnings: [],
                  forceAllowed: true,
                },
                {
                  worktree: blockedTree,
                  sessions: [liveChat],
                  archived: [],
                  commit: "c".repeat(40),
                  push: noRemote,
                  blockers: ["存在运行中的 WebMux 会话"],
                  warnings: [],
                  forceAllowed: true,
                },
              ]
            : [
                {
                  worktree: blockedTree,
                  sessions: [],
                  archived: [],
                  commit: "c".repeat(40),
                  push: noRemote,
                  blockers: [],
                  warnings: [],
                  forceAllowed: true,
                },
              ],
      });
    }
    return Response.json({
      results: [
        {
          worktreeId: unpinnedClean.id,
          path: unpinnedClean.path,
          branch: "wt/clean-feature",
          status: "removed",
          error: "",
        },
        {
          worktreeId: blockedTree.id,
          path: blockedTree.path,
          branch: "wt/wip-agent",
          status: "failed",
          error: "工作树关联会话已变化，请重新读取预览",
        },
      ],
    });
  });
  vi.stubGlobal("fetch", fetch);

  const onremoved = vi.fn();
  const onclose = vi.fn();

  render(ManageWorktreesDialog, {
    repository: repo,
    sessions: [liveChat],
    archived: [],
    home: {},
    onchange: vi.fn(),
    onremoved,
    onclose,
  });

  const dialog = page.getByRole("dialog", { name: "管理工作树 · webmux" });
  await dialog.getByRole("checkbox", { name: "wt/clean-feature" }).click();
  await dialog.getByRole("checkbox", { name: "wt/wip-agent" }).click();
  await dialog.getByRole("button", { name: "下一步：预览删除 (2)" }).click();

  const previewDialog = page.getByRole("dialog", {
    name: "确认批量清理 · webmux",
  });
  await previewDialog
    .getByRole("checkbox", { name: /同时强制删除以上 1 个受阻工作树/ })
    .click();

  const forceDialog = page.getByRole("dialog", {
    name: "批量强制删除 · webmux",
  });
  await forceDialog.getByRole("button", { name: "关闭会话并删除 (2)" }).click();

  const removeBody = JSON.parse(String(fetch.mock.calls.at(-1)?.[1]?.body));
  expect(removeBody).toEqual({
    confirmationId: "conf-1",
    includeForce: true,
    closeSessions: true,
  });
  expect(onremoved).toHaveBeenCalledWith(["clean1"]);
  expect(onclose).not.toHaveBeenCalled();

  const resultDialog = page.getByRole("dialog", {
    name: "批量清理部分完成 · webmux",
  });
  await expect
    .element(resultDialog)
    .toMatchTextContent(/已删除 1 个.*失败 1 个.*工作树关联会话已变化/);
  await resultDialog.getByRole("button", { name: "重新预览重试 (1)" }).click();

  expect(JSON.parse(String(fetch.mock.calls.at(-1)?.[1]?.body))).toEqual({
    worktreeIds: ["dirty1"],
  });
});

test("the home worktree dialog's footer hands over to batch management", async () => {
  const onmanage = vi.fn();
  const onclose = vi.fn();
  render(HomeWorktreeDialog, {
    repository: repo,
    sessions: [],
    records: [],
    home: { repo1: ["main"] },
    ontoggle: vi.fn(),
    onmanage,
    onclose,
  });

  await page.getByRole("button", { name: "批量管理…" }).click();
  expect(onmanage).toHaveBeenCalledOnce();
  expect(onclose).toHaveBeenCalledOnce();
});
