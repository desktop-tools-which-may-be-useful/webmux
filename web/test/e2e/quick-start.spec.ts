import { spawnSync } from "node:child_process";
import { mkdir } from "node:fs/promises";
import { basename, join } from "node:path";
import type { APIRequestContext, Page } from "@playwright/test";
import { expect, expectTerminalLine, test } from "./fixtures";

// Quick start over the fake agent (internal/acp/testdata/fakeagent), which
// global-setup puts on PATH as claude and claude-agent-acp: its chat side
// answers "echo: text". Each test registers its own temporary repository.

async function repository(
  request: APIRequestContext,
  root: string,
  initInstruction = "",
) {
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
    data: { path: root, initInstruction },
  });
  expect(response.status(), await response.text()).toBe(201);
  return response.json() as Promise<{ id: string }>;
}

async function hybridConfig(request: APIRequestContext, name: string) {
  const response = await request.post("/api/launch-configs", {
    data: { name, icon: "claudecode", cmd: ["claude"], env: {}, hybrid: true },
  });
  expect(response.status(), await response.text()).toBe(201);
}

const phone = (page: Page) => (page.viewportSize()?.width ?? 1280) < 640;

// The sidebar: always shown on desktop, a drawer opened from the top bar on
// phones. The drawer stays open when only the mode changes.
async function sidebar(page: Page) {
  if (phone(page))
    await page.getByRole("button", { name: "打开侧栏" }).first().click();
  return page.getByRole("navigation", { name: "导航" }).last();
}

const conversation = (page: Page) =>
  page.getByRole("list", { name: "对话", exact: true });
const sideTab = (page: Page, side: "acp" | "tui") =>
  page
    .getByRole("tablist", { name: "会话视图", exact: true })
    .getByRole(
      "tab",
      side === "acp" ? { name: /^聊天/ } : { name: "终端", exact: true },
    );

test("one send creates a worktree and starts the chat; the result is an ordinary hybrid session", async ({
  page,
  request,
  workDir,
}, info) => {
  test.slow();
  await page.setViewportSize(
    info.project.use.isMobile
      ? { width: 390, height: 844 }
      : { width: 1280, height: 720 },
  );
  const root = join(workDir, "qs-repo");
  const repo = await repository(request, root, "先读 AGENTS.md");
  await hybridConfig(request, "Claude Code");

  await page.goto("/");
  const nav = await sidebar(page);
  await nav.getByRole("button", { name: "聊天模式" }).click();
  await expect(page).toHaveURL(/\/new$/);
  await expect(nav.getByRole("button", { name: "聊天模式" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  if (phone(page)) {
    // Only the mode changed, so the drawer is still open.
    await expect(nav.getByRole("list", { name: "聊天历史" })).toHaveCount(0);
    await expect(nav.getByText("还没有聊天")).toBeVisible();
    await page.keyboard.press("Escape");
  }

  const toolbar = page.getByRole("toolbar", { name: "启动选项" });
  await expect(
    toolbar.getByRole("button", { name: "仓库：qs-repo" }),
  ).toBeVisible();
  await expect(
    toolbar.getByRole("button", { name: "基准分支：main" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "附带 qs-repo 的初始化指令" }),
  ).toBeVisible();
  const input = page.getByRole("textbox", { name: "任务描述" });
  await input.fill("修复首页排序");
  await page.getByRole("button", { name: "发送", exact: true }).click();

  await expect(page).toHaveURL(/\/s\/[a-z2-7]{8}$/, { timeout: 30_000 });
  const id = page.url().split("/").at(-1)!;
  // The first message is the instruction, a blank line and the task.
  await expect(conversation(page)).toContainText("先读 AGENTS.md");
  await expect(conversation(page)).toContainText("修复首页排序");
  await expect(conversation(page)).toContainText("echo: 先读 AGENTS.md");
  // A new linked worktree holds the session.
  const listed = await (
    await request.get(`/api/repositories/${repo.id}/worktrees`)
  ).json();
  expect(listed.worktrees).toHaveLength(2);
  const session = await (await request.get(`/api/sessions/${id}`)).json();
  expect(session.hybrid.side).toBe("acp");
  expect(session.workspace.worktreeId).not.toBe(listed.worktrees[0].id);

  // The chat history lists it as the current chat.
  const chats = (await sidebar(page)).getByRole("list", { name: "聊天历史" });
  await expect(chats.locator('[aria-current="page"]')).toHaveCount(1);
  if (phone(page)) await page.keyboard.press("Escape");

  // Switching sides works as for any hybrid session.
  await sideTab(page, "tui").click();
  await expect(sideTab(page, "tui")).toHaveAttribute("aria-selected", "true");
  await expectTerminalLine(page, "> 先读 AGENTS.md");
  await expect(sideTab(page, "acp")).toHaveAccessibleName("聊天", {
    timeout: 20_000,
  });
  await sideTab(page, "acp").click();
  await expect(sideTab(page, "acp")).toHaveAttribute("aria-selected", "true");
  await expect(conversation(page)).toContainText("echo: 先读 AGENTS.md");

  // Archived, it stays in the chat history and resumes from there.
  await page.getByRole("button", { name: "会话菜单", exact: true }).click();
  await page.getByRole("menuitem", { name: "归档会话", exact: true }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "归档", exact: true })
    .click();
  await expect(page).toHaveURL(/\/new$/);
  const history = (await sidebar(page)).getByRole("list", { name: "聊天历史" });
  const archived = history.getByRole("button", { name: /（已结束）$/ });
  await expect(archived).toHaveCount(1);
  // The workbench home shows the same record in its worktree group.
  if (phone(page)) await page.keyboard.press("Escape");
  const nav2 = await sidebar(page);
  await nav2.getByRole("button", { name: "工作台模式" }).click();
  await expect(page).toHaveURL(/\/$/);
  if (phone(page)) await page.keyboard.press("Escape");
  await expect(
    page.getByRole("region", { name: /的已归档会话$/ }).first(),
  ).toBeVisible();
  // Back in chat mode, the record resumes on its chat side.
  const nav3 = await sidebar(page);
  await nav3.getByRole("button", { name: "聊天模式" }).click();
  await nav3
    .getByRole("list", { name: "聊天历史" })
    .getByRole("button", { name: /（已结束）$/ })
    .click();
  await expect(page).toHaveURL(new RegExp(`/s/${id}$`));
  await expect(conversation(page)).toContainText("修复首页排序");
  expect(basename(root)).toBe("qs-repo");
});

test("phone drawer: the chat history switches sessions and closes", async ({
  page,
  request,
  workDir,
}, info) => {
  test.skip(!info.project.use.isMobile, "the drawer is the phone sidebar");
  await page.setViewportSize({ width: 390, height: 844 });
  await hybridConfig(request, "Claude Code");
  const configs = await (await request.get("/api/launch-configs")).json();
  const ids: string[] = [];
  for (const name of ["第一个聊天", "第二个聊天"]) {
    const response = await request.post("/api/sessions", {
      data: { cwd: workDir, configId: configs[0].id, startSide: "acp", name },
    });
    expect(response.status(), await response.text()).toBe(201);
    ids.push((await response.json()).id);
  }
  await page.goto(`/s/${ids[0]}`);
  const open = page.getByRole("button", { name: "打开侧栏" });
  await open.click();
  const drawer = page.getByRole("dialog", { name: "导航" });
  await expect(drawer).toBeVisible();
  await drawer.getByRole("button", { name: "聊天模式" }).click();
  // Only the mode changed: the drawer stays open on the chat history.
  await expect(drawer).toBeVisible();
  await drawer.getByRole("button", { name: /^第二个聊天/ }).click();
  await expect(drawer).toBeHidden();
  await expect(page).toHaveURL(new RegExp(`/s/${ids[1]}$`));
  // The top bar stays on one line.
  const header = page.locator(".terminal-header").first();
  expect((await header.boundingBox())!.height).toBeLessThanOrEqual(48);
});
