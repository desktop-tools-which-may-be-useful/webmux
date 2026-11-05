import { afterEach, beforeEach, expect, test } from "vitest";
import { page, userEvent } from "vitest/browser";
import { render } from "vitest-browser-svelte/pure";
import { shellUi } from "../../src/shell.svelte";
import { createTerminalDrafts } from "../../src/terminal-draft.svelte";
import {
  commands,
  installChatFixture,
  respond,
  socketAt,
} from "./chat-fixture";
import ShellHarness from "./ShellHarness.svelte";
import { launchConfig, repository, worktree } from "./shell-fixture";

installChatFixture();

const effort = {
  id: "effort",
  name: "思考",
  category: "thought_level",
  type: "select" as const,
  value: "medium",
  options: [
    { value: "medium", name: "中" },
    { value: "high", name: "高" },
  ],
};
const branches = {
  branches: [
    { name: "main", head: "a".repeat(40), worktreePaths: ["/repos/webmux"] },
    { name: "feature", head: "b".repeat(40), worktreePaths: [] },
  ],
  bases: [],
  defaultBase: "a".repeat(40),
  defaultBaseSubject: "init",
  defaultBranch: "main",
  defaultParent: "/repos/webmux.worktrees",
};

beforeEach(() => {
  shellUi.setMode("chat");
  history.replaceState(null, "", "/new");
  respond(/^GET \/api\/repositories$/, () =>
    Response.json([
      repository({
        id: "gone",
        name: "legacy-api",
        availability: "unavailable",
        error: "路径不可用",
        worktrees: [],
      }),
      repository({ initInstruction: "先读 AGENTS.md" }),
    ]),
  );
  respond(/^GET \/api\/repositories\/repo1\/branches$/, () =>
    Response.json(branches),
  );
  respond(/^GET \/api\/launch-configs$/, () =>
    Response.json([
      launchConfig({ id: "cfgplain", name: "Shell", hybrid: false }),
      launchConfig({ chatOptions: [effort] }),
      launchConfig({ id: "cfgcodex", name: "Codex", icon: "codex" }),
    ]),
  );
  respond(/^GET \/api\/hybrid-sessions/, () => Response.json([]));
});
afterEach(() => {
  for (const key of ["shellMode", "quickStart", "quick-start-draft"])
    localStorage.removeItem(`webmux.${key}`);
  history.replaceState(null, "", "/");
});

async function open() {
  render(ShellHarness, { drafts: createTerminalDrafts() });
  const socket = await socketAt("/ws/sessions");
  socket.frame({ type: "snapshot", sessions: [], forwards: [] } as never);
  const input = page.getByRole("textbox", { name: "任务描述" });
  await expect.element(input).toHaveFocus();
  const toolbar = page.getByRole("toolbar", { name: "启动选项" });
  // Branches arrive after the repository list.
  await expect
    .element(toolbar.getByRole("button", { name: "基准分支：main" }))
    .toBeVisible();
  return { input, toolbar };
}

test("defaults skip what is unusable, and the first send goes in one request", async () => {
  respond(/^POST \/api\/quick-starts$/, () =>
    Response.json(
      {
        sessionId: "chataaaa",
        worktree: worktree({ id: "otter", kind: "linked" }),
        configWarnings: [],
      },
      { status: 201 },
    ),
  );
  const { input, toolbar } = await open();
  // The unavailable repository is skipped; only hybrid configs are agents.
  await expect
    .element(toolbar.getByRole("button", { name: "仓库：webmux" }))
    .toBeVisible();
  await expect
    .element(toolbar.getByRole("button", { name: "Agent：Claude Code" }))
    .toBeVisible();
  await toolbar.getByRole("button", { name: "Agent：Claude Code" }).click();
  await expect
    .element(page.getByRole("menuitemradio", { name: /Codex/ }))
    .toBeVisible();
  expect(page.getByRole("menuitemradio").elements()).toHaveLength(2);
  await userEvent.keyboard("{Escape}");
  // The init instruction is announced above the input.
  await expect
    .element(page.getByRole("button", { name: "附带 webmux 的初始化指令" }))
    .toBeVisible();
  // The remembered option catalog is editable before the agent starts.
  await toolbar.getByRole("button", { name: "思考：中" }).click();
  await page.getByRole("menuitemradio", { name: "高" }).click();

  await input.fill("修复首页排序");
  await userEvent.keyboard("{Enter}");
  await expect.poll(() => location.pathname).toBe("/s/chataaaa");
  const sent = commands().find((c) => c.path === "/api/quick-starts");
  expect(sent?.body).toMatchObject({
    repositoryId: "repo1",
    configId: "cfgclaud",
    base: "a".repeat(40),
    branch: expect.stringMatching(/^wt\/[a-z]+-[a-z]+$/),
    text: "修复首页排序",
    config: { effort: "high" },
    permissionMode: "ask",
  });
  expect(JSON.parse(localStorage.getItem("webmux.quickStart")!)).toEqual({
    repositoryId: "repo1",
    configId: "cfgclaud",
  });
  expect(localStorage.getItem("webmux.quick-start-draft")).toBeNull();
});

test("a remembered agent that is gone falls back to the first hybrid config", async () => {
  localStorage.setItem(
    "webmux.quickStart",
    JSON.stringify({ repositoryId: "gone", configId: "cfgdeled" }),
  );
  const { toolbar } = await open();
  await expect
    .element(toolbar.getByRole("button", { name: "Agent：Claude Code" }))
    .toBeVisible();
  await expect
    .element(toolbar.getByRole("button", { name: "仓库：webmux" }))
    .toBeVisible();
});

test("a failed step keeps the text and offers what was created", async () => {
  respond(/^POST \/api\/quick-starts$/, () =>
    Response.json(
      {
        error: "claude-agent-acp 在初始化阶段退出（exit 1）",
        step: "session",
        worktree: worktree({
          id: "otter",
          kind: "linked",
          path: "/repos/webmux.worktrees/brisk-otter",
        }),
      },
      { status: 502 },
    ),
  );
  const { input } = await open();
  await input.fill("修复首页排序");
  await page.getByRole("button", { name: "发送", exact: true }).click();
  const alert = page.getByRole("alert");
  await expect
    .element(alert)
    .toMatchTextContent(/启动 agent 失败：claude-agent-acp 在初始化阶段退出/);
  await expect
    .element(alert)
    .toMatchTextContent(
      /已创建 worktree：\/repos\/webmux.worktrees\/brisk-otter/,
    );
  await expect
    .element(alert.getByRole("button", { name: "在此 worktree 新建会话" }))
    .toBeVisible();
  await expect.element(input).toHaveValue("修复首页排序");
  expect(location.pathname).toBe("/new");
});

test("without hybrid configs the send stays off and says why", async () => {
  respond(/^GET \/api\/launch-configs$/, () =>
    Response.json([launchConfig({ id: "cfgplain", hybrid: false })]),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await expect
    .element(page.getByText(/还没有勾选混合模式的启动配置/))
    .toBeVisible();
  await page.getByRole("textbox", { name: "任务描述" }).fill("修复");
  await expect
    .element(page.getByRole("button", { name: "发送", exact: true }))
    .toBeDisabled();
  await expect
    .element(page.getByRole("link", { name: "新建启动配置" }))
    .toBeVisible();
});
