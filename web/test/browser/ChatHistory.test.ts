import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import ChatHistory from "../../src/components/ChatHistory.svelte";
import {
  chat,
  launchConfig,
  record,
  repository,
  session,
  worktree,
} from "./shell-fixture";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
});

const list = () => page.getByRole("list", { name: "聊天历史" });
const archivedList = () => page.getByRole("list", { name: "已归档聊天" });

test("live chats show newest first and archived records fold below", async () => {
  const onopen = vi.fn();
  const onresume = vi.fn();
  const working = chat({
    id: "chataaaa",
    title: "工作中的聊天",
    status: "running",
    lastActiveAt: "2026-10-28T03:00:00Z",
  });
  render(ChatHistory, {
    sessions: [
      working,
      chat({
        id: "chatbbbb",
        title: "等授权",
        status: "waiting",
        lastActiveAt: "2026-10-28T02:00:00Z",
        attention: {
          kind: "permission",
          source: "agent",
          since: "2026-10-28T02:00:00Z",
        },
      }),
      chat({
        id: "chatcccc",
        title: "空闲的聊天",
        lastActiveAt: "2026-10-28T01:00:00Z",
      }),
      // Terminal sessions never show here.
      session({ id: "shellaaa", title: "pnpm dev" }),
    ],
    records: [
      record({ id: "chatcccc", title: "旧记录" }),
      record({
        id: "archivea",
        title: "整理文档",
        updatedAt: "2026-10-28T02:30:00Z",
      }),
    ],
    configs: [launchConfig()],
    repositories: [],
    current: "chatbbbb",
    onopen,
    onresume,
    onrename: vi.fn(),
    onclose: vi.fn(),
    onrelease: vi.fn(),
    onremove: vi.fn(),
  });
  const rows = list().getByRole("button", { name: /（/ });
  await expect.element(rows.first()).toBeVisible();
  expect(rows.elements().map((row) => row.getAttribute("aria-label"))).toEqual([
    "工作中的聊天（工作中）",
    "等授权（等待）",
    "空闲的聊天（空闲）",
  ]);
  await expect
    .element(list().getByRole("button", { name: "等授权（等待）" }))
    .toHaveAttribute("aria-current", "page");
  await list().getByRole("button", { name: "工作中的聊天（工作中）" }).click();
  expect(onopen).toHaveBeenCalledWith(working);

  // Archived records are collapsed by default; expanding shows them.
  const trigger = page.getByRole("button", { name: /已归档/ });
  await expect.element(trigger).toHaveAttribute("aria-expanded", "false");
  await trigger.click();
  await expect.element(trigger).toHaveAttribute("aria-expanded", "true");
  await archivedList()
    .getByRole("button", { name: "整理文档（已结束）" })
    .click();
  expect(onresume).toHaveBeenCalledWith(
    expect.objectContaining({ id: "archivea" }),
  );
});

test("archived section opens automatically when viewing or resuming an archived record", async () => {
  render(ChatHistory, {
    sessions: [],
    records: [
      record({
        id: "archivea",
        title: "整理文档",
        updatedAt: "2026-10-28T02:30:00Z",
      }),
    ],
    configs: [launchConfig()],
    repositories: [],
    current: "archivea",
    onopen: vi.fn(),
    onresume: vi.fn(),
    onrename: vi.fn(),
    onclose: vi.fn(),
    onrelease: vi.fn(),
    onremove: vi.fn(),
  });
  const trigger = page.getByRole("button", { name: /已归档/ });
  await expect.element(trigger).toHaveAttribute("aria-expanded", "true");
  await expect
    .element(archivedList().getByRole("button", { name: "整理文档（已结束）" }))
    .toHaveAttribute("aria-current", "page");
});

test("archived rows offer releasing a linked worktree and mark released records", async () => {
  const linked = worktree({
    id: "wt1",
    path: "/repos/webmux.worktrees/wt1",
    kind: "linked",
    branch: "wt/fix-sort",
  });
  const onrelease = vi.fn();
  const onresume = vi.fn();
  const released = record({
    id: "archiver",
    title: "已释放的聊天",
    cwd: "/repos/webmux.worktrees/old",
    released: {
      id: "rel1",
      repositoryId: "repo1",
      path: "/repos/webmux.worktrees/old",
      commit: "d".repeat(40),
      releasedAt: "2026-10-28T00:00:00Z",
    },
  });
  render(ChatHistory, {
    sessions: [],
    records: [
      record({
        id: "archivew",
        title: "分支聊天",
        cwd: linked.path,
        workspace: {
          repositoryId: "repo1",
          worktreeId: "wt1",
          worktreePath: linked.path,
        },
        updatedAt: "2026-10-28T02:00:00Z",
      }),
      record({ id: "archivem", title: "主工作区聊天" }),
      released,
    ],
    configs: [launchConfig()],
    repositories: [repository({ worktrees: [worktree(), linked] })],
    current: "",
    onopen: vi.fn(),
    onresume,
    onrename: vi.fn(),
    onclose: vi.fn(),
    onrelease,
    onremove: vi.fn(),
  });
  await page.getByRole("button", { name: /已归档/ }).click();
  const menuItems = async (title: string) => {
    await archivedList()
      .getByRole("button", { name: `会话操作：${title}` })
      .click();
    await expect.element(page.getByRole("menu")).toBeVisible();
    const items = page
      .getByRole("menuitem")
      .elements()
      .map((item) => item.textContent?.trim());
    await userEvent.keyboard("{Escape}");
    return items;
  };
  expect(await menuItems("分支聊天")).toEqual([
    "恢复聊天",
    "释放空间",
    "删除记录",
  ]);
  expect(await menuItems("主工作区聊天")).toEqual(["恢复聊天", "删除记录"]);
  expect(await menuItems("已释放的聊天")).toEqual(["恢复聊天", "删除记录"]);

  await archivedList()
    .getByRole("button", { name: "会话操作：分支聊天" })
    .click();
  await page.getByRole("menuitem", { name: "释放空间" }).click();
  expect(onrelease).toHaveBeenCalledWith(linked);

  // The released row says so, with its path and commit on hover, and still
  // resumes through the caller, which confirms the rebuild first.
  const row = archivedList().getByRole("button", {
    name: "已释放的聊天（已释放空间，已结束）",
  });
  const badge = row.getByTitle(
    `原路径 /repos/webmux.worktrees/old · 提交 ${"d".repeat(12)}`,
    { exact: true },
  );
  await expect.element(badge).toBeVisible();
  expect(badge.element().textContent).toContain("已释放空间");
  await row.click();
  expect(onresume).toHaveBeenCalledWith(released);
});
