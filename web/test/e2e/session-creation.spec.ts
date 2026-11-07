import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir } from "node:fs/promises";
import { join, resolve } from "node:path";
import type { APIRequestContext, Page } from "@playwright/test";
import {
  chooseLaunch,
  chooseLocation,
  expect,
  openPaneMenu,
  test,
  useCustomCommand,
} from "./fixtures";
import { repositoryMenu } from "./repository-menu";

const exec = promisify(execFile);

async function register(request: APIRequestContext, path: string) {
  await mkdir(path);
  await exec("git", ["-C", path, "init", "-b", "main"]);
  await exec("git", [
    "-C",
    path,
    "-c",
    "user.name=QA",
    "-c",
    "user.email=qa@example.invalid",
    "commit",
    "--allow-empty",
    "-m",
    "initial",
  ]);
  const response = await request.post("/api/repositories", { data: { path } });
  expect(response.status()).toBe(201);
  return response.json();
}

// Picks a home view in the workbench tree (sidebar or phone drawer).
async function selectScope(page: Page, id: string) {
  const tree = page.getByRole("tree", { name: "仓库与会话" });
  const open = page.getByRole("button", { name: "打开侧栏" });
  await expect(tree.or(open).first()).toBeVisible();
  if (!(await tree.isVisible()))
    await page.getByRole("button", { name: "打开侧栏" }).click();
  const key = id === "all" || id === "direct" ? id : `r:${id}`;
  await tree.locator(`[data-tree-key="${key}"]`).click();
}

test("global and worktree creation use the selected repository instead of an unrelated recent directory", async ({
  page,
  request,
  workDir,
}) => {
  await register(request, join(workDir, "first"));
  const second = await register(request, join(workDir, "second"));
  await page.addInitScript(
    (path) => localStorage.setItem("webmux.recentDirs", JSON.stringify([path])),
    workDir,
  );
  await page.goto("/");
  await selectScope(page, second.id);
  await page
    .getByRole("main")
    .getByRole("button", { name: "＋ 新建会话", exact: true })
    .first()
    .click();
  const location = page.getByRole("button", { name: "位置", exact: true });
  await expect(location).toContainText("second · 主工作区 · main");
  await page.getByRole("button", { name: "取消", exact: true }).click();
  expect(await (await request.get("/api/sessions")).json()).toEqual([]);
  const group = page.locator(`[data-worktree-id="${second.worktrees[0].id}"]`);
  await group.getByRole("button", { name: "新建会话", exact: true }).click();
  await expect(location).toContainText("second · 主工作区 · main");
  await useCustomCommand(page, "/bin/sh -i");
  await page.getByRole("button", { name: "创建并进入 →", exact: true }).click();
  await expect(page).toHaveURL(/\/s\/[a-z0-9]+$/);
  const sessions = await (await request.get("/api/sessions")).json();
  expect(sessions).toHaveLength(1);
  expect(sessions[0]).toMatchObject({
    cwd: second.anchorPath,
    workspace: { repositoryId: second.id, worktreeId: second.worktrees[0].id },
  });
});

test("worktree management creates only a worktree even when launch configs are unavailable", async ({
  page,
  request,
  workDir,
}) => {
  const repo = await register(request, join(workDir, "project"));
  await page.route("**/api/launch-configs", (route) =>
    route.fulfill({ status: 503, json: { error: "launch store offline" } }),
  );
  await page.goto("/");
  await expect(
    page.getByRole("button", { name: "新建 worktree", exact: true }),
  ).toHaveCount(0);
  await repositoryMenu(page, "新建工作树…");
  await expect(
    page.getByRole("checkbox", { name: "创建后启动会话", exact: true }),
  ).not.toBeChecked();
  await expect(
    page.getByRole("button", { name: "运行", exact: true }),
  ).toHaveCount(0);
  const branch = page.getByLabel("新分支名称", { exact: true });
  await expect(branch).not.toHaveValue("");
  await branch.fill("feature/prepared-only");
  await page.getByRole("button", { name: "创建工作树", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page).toHaveURL(/\/$/);
  expect(await (await request.get("/api/sessions")).json()).toEqual([]);
  const repositories = await (await request.get("/api/repositories")).json();
  expect(
    repositories.find((item: { id: string }) => item.id === repo.id).worktrees,
  ).toHaveLength(2);
});

test("full split creation uses saved configs and keeps the launch subdirectory and original terminal", async ({
  page,
  request,
  workDir,
}) => {
  const repo = await register(request, join(workDir, "project"));
  const cwd = join(repo.anchorPath, "packages", "app");
  await mkdir(cwd, { recursive: true });
  const response = await request.post("/api/sessions", {
    data: { cwd, cmd: ["/bin/sh", "-i"], name: "Original" },
  });
  const original = (await response.json()).session;
  expect(original.workspace.repositoryId).toBe(repo.id);
  const configResponse = await request.post("/api/launch-configs", {
    data: {
      name: "Project shell",
      cmd: ["/bin/sh", "-i"],
      env: { SPLIT_TEST: "saved" },
    },
  });
  expect(configResponse.status()).toBe(201);
  const config = await configResponse.json();
  await page.addInitScript(
    (path) => localStorage.setItem("webmux.recentDirs", JSON.stringify([path])),
    workDir,
  );
  await page.goto(`/s/${original.id}`);
  const pane = page.locator(`[data-session-id="${original.id}"]`);
  await expect(pane.locator(".xterm-helper-textarea")).toBeAttached();
  await pane
    .locator(".xterm")
    .evaluate((element) => element.setAttribute("data-retained", "yes"));
  const open = async () => {
    await openPaneMenu(pane);
    await page
      .getByRole("menuitem", { name: "新建会话…", exact: true })
      .click();
    await expect(page.getByLabel("工作目录", { exact: true })).toHaveValue(cwd);
  };
  await open();
  await page.getByRole("button", { name: "取消", exact: true }).click();
  expect(await (await request.get("/api/sessions")).json()).toHaveLength(1);
  await open();
  await chooseLaunch(page, "Project shell");
  await page.getByRole("radio", { name: "上下分屏", exact: true }).click();
  await page
    .getByRole("button", { name: "创建并加入分屏 →", exact: true })
    .click();
  await expect(page.locator(".split-terminal")).toHaveCount(2);
  await expect(page).toHaveURL(new RegExp(`/s/${original.id}$`));
  await expect(pane.locator(".xterm")).toHaveAttribute("data-retained", "yes");
  const sessions = await (await request.get("/api/sessions")).json();
  expect(
    sessions.find((item: { id: string }) => item.id !== original.id),
  ).toMatchObject({
    cwd,
    configId: config.id,
    workspace: { repositoryId: repo.id },
  });
  await openPaneMenu(page.locator(".split-terminal").last());
  await page.getByRole("menuitem", { name: "左右分屏 · 新建 Shell" }).click();
  await expect(page.locator(".split-terminal")).toHaveCount(3);
  const all = await (await request.get("/api/sessions")).json();
  expect(all.every((item: { cwd: string }) => item.cwd === cwd)).toBe(true);
});

test("split creation retains a new worktree across repository changes after command failure", async ({
  page,
  request,
  workDir,
}, testInfo) => {
  const repo = await register(request, join(workDir, "project"));
  const other = await register(request, join(workDir, "other"));
  const response = await request.post("/api/sessions", {
    data: { cwd: repo.anchorPath, cmd: ["/bin/sh", "-i"], name: "Original" },
  });
  const original = (await response.json()).session;
  await page.goto(`/s/${original.id}`);
  await openPaneMenu(page);
  await page.getByRole("menuitem", { name: "新建会话…", exact: true }).click();
  // The pane's repository is listed first, so its "new worktree" comes first.
  await chooseLocation(page, "新建工作树…");
  const location = page.getByRole("button", { name: "位置", exact: true });
  await expect(location).toContainText("新建工作树 · project");
  const branch = page.getByLabel("新分支名称", { exact: true });
  await expect(branch).not.toHaveValue("");
  await branch.fill("feature/retry-target");
  await page.getByRole("button", { name: "高级：起始提交与存放目录" }).click();
  const pathInput = page.getByLabel("新工作树完整路径", { exact: true });
  await expect(pathInput).toHaveValue(/\/retry-target$/);
  const path = await pathInput.inputValue();
  await useCustomCommand(page, "/webmux-missing-agent");
  await page
    .getByRole("button", { name: "创建并加入分屏 →", exact: true })
    .click();
  await expect(page.getByRole("alert")).toContainText("webmux-missing-agent");
  await expect(location).toContainText("project · feature/retry-target");
  await chooseLocation(page, `主工作区 · main ${other.anchorPath}`);
  await expect(location).not.toContainText("retry-target");
  await location.click();
  const retained = page.getByRole("option", {
    name: /^feature\/retry-target/,
  });
  await expect(retained).toBeVisible();
  const evidence = process.env.WEBMUX_SESSION_RECOVERY_EVIDENCE_DIR;
  if (evidence) {
    await mkdir(resolve(evidence), { recursive: true });
    for (const theme of ["light", "dark"]) {
      await page.evaluate(async (theme) => {
        document.documentElement.classList.toggle("dark", theme === "dark");
        await document.fonts.ready;
      }, theme);
      await page.screenshot({
        path: resolve(
          evidence,
          `recovery-${testInfo.project.name}-${theme}.jpg`,
        ),
        type: "jpeg",
        quality: 75,
        scale: "css",
        fullPage: true,
        animations: "disabled",
      });
    }
  }
  await retained.click();
  await page.getByLabel("自定义启动命令", { exact: true }).fill("/bin/sh -i");
  await page
    .getByRole("button", { name: "创建并加入分屏 →", exact: true })
    .click();
  await expect(page.locator(".split-terminal")).toHaveCount(2);
  await expect(page).toHaveURL(new RegExp(`/s/${original.id}$`));
  const sessions = await (await request.get("/api/sessions")).json();
  expect(sessions).toHaveLength(2);
  expect(
    sessions.find((item: { id: string }) => item.id !== original.id),
  ).toMatchObject({ cwd: path, workspace: { repositoryId: repo.id } });
  const repositories = await (await request.get("/api/repositories")).json();
  expect(
    repositories.find((item: { id: string }) => item.id === repo.id).worktrees,
  ).toHaveLength(2);
});
