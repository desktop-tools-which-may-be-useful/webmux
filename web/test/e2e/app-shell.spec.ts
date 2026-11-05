import { spawnSync } from "node:child_process";
import { mkdir } from "node:fs/promises";
import { join } from "node:path";
import type { APIRequestContext, Page } from "@playwright/test";
import { expect, expectConnected, test } from "./fixtures";

// The app shell around the home page, quick start and session pages: the
// sidebar beside a session, its workbench tree, rename, and the phone
// drawer. Sessions run plain shells or the fake agent (see hybrid.spec.ts).

async function repository(request: APIRequestContext, root: string) {
  await mkdir(root);
  for (const args of [
    ["init", "-b", "main"],
    [
      "-c",
      "user.name=WebMux QA",
      "-c",
      "user.email=qa@example.invalid",
      "commit",
      "--allow-empty",
      "-m",
      "initial",
    ],
  ])
    expect(spawnSync("git", ["-C", root, ...args]).status).toBe(0);
  const response = await request.post("/api/repositories", {
    data: { path: root },
  });
  expect(response.status(), await response.text()).toBe(201);
  return (await response.json()) as {
    id: string;
    worktrees: { id: string }[];
  };
}

async function shell(
  request: APIRequestContext,
  name: string,
  data: Record<string, unknown>,
) {
  const response = await request.post("/api/sessions", {
    data: { cmd: ["/bin/sh", "-c", "sleep 600"], name, ...data },
  });
  expect(response.status(), await response.text()).toBe(201);
  return (await response.json()).id as string;
}

async function chat(request: APIRequestContext, cwd: string, name: string) {
  const config = await request.post("/api/launch-configs", {
    data: { name: "Claude Code", cmd: ["claude"], env: {}, hybrid: true },
  });
  expect(config.status(), await config.text()).toBe(201);
  const response = await request.post("/api/sessions", {
    data: { cwd, configId: (await config.json()).id, startSide: "acp", name },
  });
  expect(response.status(), await response.text()).toBe(201);
  return (await response.json()).id as string;
}

const tree = (page: Page) => page.getByRole("tree", { name: "仓库与会话" });
const cols = async (request: APIRequestContext, id: string) =>
  (await (await request.get(`/api/sessions/${id}`)).json()).cols as number;

test.describe("desktop sidebar", () => {
  test.skip(({ isMobile }) => isMobile, "the desktop sidebar");

  test("collapsing the sidebar widens the terminal and keeps it in the viewport", async ({
    page,
    request,
    workDir,
  }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
    const id = await shell(request, "Columns", {
      cwd: workDir,
      cmd: ["/bin/bash", "--noprofile", "--norc", "-i"],
    });
    await page.goto(`/s/${id}`);
    await expectConnected(page);
    const fits = () =>
      page.evaluate(() => {
        const box = document
          .querySelector(".terminal-page")!
          .getBoundingClientRect();
        return box.right <= innerWidth + 0.5 && box.left > 0;
      });
    expect(await fits()).toBe(true);
    await expect.poll(() => cols(request, id)).toBeGreaterThan(0);
    const wide = await cols(request, id);
    await page.getByRole("button", { name: "收起侧栏" }).click();
    await expect.poll(() => cols(request, id)).toBeGreaterThan(wide);
    expect(await fits()).toBe(true);
    await page.reload();
    await expect(page.getByRole("button", { name: "展开侧栏" })).toBeVisible();
    await page.getByRole("button", { name: "展开侧栏" }).click();
    await expect.poll(() => cols(request, id)).toBe(wide);
  });

  test("the tree jumps to worktrees, opens sessions by their side and renames them", async ({
    page,
    request,
    workDir,
  }) => {
    test.slow();
    const repo = await repository(request, join(workDir, "tree-repo"));
    const main = repo.worktrees[0]!.id;
    const terminal = await shell(request, "pnpm dev", {
      cwd: join(workDir, "tree-repo"),
      workspace: { repositoryId: repo.id, worktreeId: main },
    });
    const agent = await chat(request, join(workDir, "tree-repo"), "修复排序");
    await page.goto(`/s/${terminal}`);
    // The current session's repository and worktree open on their own.
    await expect(
      tree(page).getByRole("treeitem", { name: /^pnpm dev（/ }),
    ).toHaveAttribute("aria-current", "page");

    // A terminal session keeps the workbench and shows its terminal.
    await tree(page).getByRole("treeitem", { name: /^main/ }).click();
    await expect(page).toHaveURL(new RegExp(`/#wt-${main}$`));
    await expect(page.locator(`#wt-${main} h3`)).toBeFocused();
    // The home view has no current session, so its worktree is folded.
    await tree(page).getByRole("button", { name: "展开main" }).click();
    await tree(page)
      .getByRole("treeitem", { name: /^pnpm dev（/ })
      .click();
    await expect(page).toHaveURL(new RegExp(`/s/${terminal}$`));
    await expect(
      page.getByRole("button", { name: "工作台模式" }),
    ).toHaveAttribute("aria-pressed", "true");

    // A chat-side hybrid session takes the shell to chat mode.
    await page.goto("/");
    await page.getByRole("link", { name: "修复排序" }).click();
    await expect(page).toHaveURL(new RegExp(`/s/${agent}$`));
    await expect(
      page.getByRole("button", { name: "聊天模式" }),
    ).toHaveAttribute("aria-pressed", "true");
    const chats = page.getByRole("list", { name: "聊天历史" });
    await expect(chats.locator('[aria-current="page"]')).toHaveCount(1);

    // Renamed from the sidebar, the title follows everywhere.
    await chats.getByRole("button", { name: "修改标题：修复排序" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox").fill("首页排序");
    await page.keyboard.press("Enter");
    await expect(
      chats.getByRole("button", { name: /^首页排序（/ }),
    ).toBeVisible();
    await expect(page.locator(".terminal-header")).toContainText("首页排序");
    await chats.getByRole("button", { name: "修改标题：首页排序" }).click();
    await dialog.getByRole("textbox").fill("");
    await dialog.getByRole("button", { name: "保存" }).click();
    await expect(
      chats.getByRole("button", { name: /^首页排序（/ }),
    ).toHaveCount(0);

    // The workbench "+" opens the existing new-worktree dialog in place.
    await page.getByRole("button", { name: "工作台模式" }).click();
    await expect(page).toHaveURL(new RegExp(`/s/${agent}$`));
    await tree(page)
      .getByRole("treeitem", { name: /^tree-repo，/ })
      .hover();
    await tree(page)
      .getByRole("button", { name: "在 tree-repo 新建工作树" })
      .click();
    const create = page.getByRole("dialog");
    await expect(create).toBeVisible();
    await create.getByRole("button", { name: /^创建工作树/ }).click();
    await expect(create).toBeHidden();
    await expect(page).toHaveURL(new RegExp(`/s/${agent}$`));
    await expect(
      tree(page).getByRole("treeitem", { name: /^wt\// }),
    ).toHaveCount(1);
  });
});

test.describe("phone drawer", () => {
  test.skip(({ isMobile }) => !isMobile, "the phone drawer");

  for (const size of [
    { width: 390, height: 844 },
    { width: 320, height: 568 },
  ])
    test(`the session top bar stays one line and opens the drawer at ${size.width}px`, async ({
      page,
      request,
      workDir,
    }) => {
      await page.setViewportSize(size);
      const id = await shell(request, "demo-agent", { cwd: workDir });
      await page.goto(`/s/${id}`);
      const header = page.locator(".terminal-header").first();
      await expect(header).toBeVisible();
      expect((await header.boundingBox())!.height).toBeLessThanOrEqual(48);
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth),
      ).toBeLessThanOrEqual(size.width + 1);
      const open = header.getByRole("button", { name: "打开侧栏" });
      await open.click();
      const drawer = page.getByRole("dialog", { name: "导航" });
      await expect(drawer).toBeVisible();
      await expect(drawer.locator(":focus")).toHaveCount(1);
      await page.keyboard.press("Escape");
      await expect(drawer).toBeHidden();
      await expect(open).toBeFocused();
      // "全部仓库" in the drawer returns to the workbench home.
      await open.click();
      await drawer.locator('[data-tree-key="all"]').click();
      await expect(drawer).toBeHidden();
      await expect(page).toHaveURL(/\/$/);
    });
});
