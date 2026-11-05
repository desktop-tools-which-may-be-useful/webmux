import { afterEach, beforeEach, expect, test, vi } from "vitest";
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
    // Shares main's commit: branches are told apart by name.
    { name: "release", head: "a".repeat(40), worktreePaths: [] },
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
  // Repository and branch sit above the input.
  const place = page.getByRole("toolbar", { name: "仓库与分支" });
  // Branches arrive after the repository list.
  await expect
    .element(place.getByRole("button", { name: "基准分支：main" }))
    .toBeVisible();
  return { input, toolbar, place };
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
  const { input, toolbar, place } = await open();
  // The unavailable repository is skipped; only hybrid configs are agents.
  await expect
    .element(place.getByRole("button", { name: "仓库：webmux" }))
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
  // The init instruction is an explicit, default-ticked choice above the input.
  await expect
    .element(page.getByRole("checkbox", { name: "使用初始化指令" }))
    .toBeChecked();
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
    permissionMode: "auto",
    useInitInstruction: true,
  });
  expect(JSON.parse(localStorage.getItem("webmux.quickStart")!)).toEqual({
    repositoryId: "repo1",
    configId: "cfgclaud",
  });
  expect(localStorage.getItem("webmux.quick-start-draft")).toBeNull();
});

const created = () =>
  Response.json(
    {
      sessionId: "chataaaa",
      worktree: worktree({ id: "otter", kind: "linked" }),
      configWarnings: [],
    },
    { status: 201 },
  );

test("unticking the init instruction applies to that send only", async () => {
  respond(/^POST \/api\/quick-starts$/, created);
  const { input } = await open();
  const box = page.getByRole("checkbox", { name: "使用初始化指令" });
  await box.click();
  await expect.element(box).not.toBeChecked();
  await input.fill("修复首页排序");
  await userEvent.keyboard("{Enter}");
  await expect.poll(() => location.pathname).toBe("/s/chataaaa");
  const sent = commands().find((c) => c.path === "/api/quick-starts");
  expect(sent?.body).toMatchObject({ useInitInstruction: false });
});

test("the preview expands quick command references and flags unknown ones", async () => {
  respond(/^GET \/api\/repositories$/, () =>
    Response.json([
      repository({ initInstruction: "先 {{恢复会话}}，再 {{没有}}\n{{dir}}" }),
    ]),
  );
  await open();
  await page.getByRole("button", { name: "预览" }).click();
  await expect
    .element(page.getByText(/先 \/resume，再 \{\{没有\}\}/))
    .toBeVisible();
  await expect
    .element(page.getByText(/找不到快捷指令：\{\{没有\}\}/))
    .toBeVisible();
  await expect.element(page.getByText(/创建后替换/)).toBeVisible();
});

test("the control is hidden and false is sent without an instruction", async () => {
  respond(/^GET \/api\/repositories$/, () => Response.json([repository()]));
  respond(/^POST \/api\/quick-starts$/, created);
  const { input } = await open();
  expect(
    page.getByRole("checkbox", { name: "使用初始化指令" }).elements(),
  ).toEqual([]);
  await input.fill("修复首页排序");
  await userEvent.keyboard("{Enter}");
  await expect.poll(() => location.pathname).toBe("/s/chataaaa");
  const sent = commands().find((c) => c.path === "/api/quick-starts");
  expect(sent?.body).toMatchObject({ useInitInstruction: false });
});

test("the base branch is searched and picked by name", async () => {
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
  const { input, place } = await open();
  await place.getByRole("button", { name: "基准分支：main" }).click();
  const list = page.getByRole("listbox", { name: "本地分支列表" });
  await expect.poll(() => list.getByRole("option").elements()).toHaveLength(3);
  await page.getByRole("combobox", { name: "基准分支" }).fill("feat");
  await page.getByRole("option", { name: "feature", exact: true }).click();
  await expect
    .element(place.getByRole("button", { name: "基准分支：feature" }))
    .toBeVisible();
  await input.fill("修复首页排序");
  await userEvent.keyboard("{Enter}");
  await expect.poll(() => location.pathname).toBe("/s/chataaaa");
  const sent = commands().find((c) => c.path === "/api/quick-starts");
  expect(sent?.body).toMatchObject({ base: "b".repeat(40) });
});

test("choosing an agent reads its options and holds the row until done", async () => {
  const pending = Promise.withResolvers<Response>();
  respond(/^POST \/api\/quick-starts\/options$/, (request) =>
    (request.body as { configId: string }).configId === "cfgcodex"
      ? pending.promise
      : Response.json({ error: "agent 已退出" }, { status: 502 }),
  );
  const { input, toolbar } = await open();
  // A failed read keeps the remembered options and offers no retry.
  await expect
    .element(toolbar.getByRole("button", { name: "思考：中" }))
    .toBeEnabled();
  expect(toolbar.getByRole("button", { name: /重新读取/ }).elements()).toEqual(
    [],
  );

  await toolbar.getByRole("button", { name: "Agent：Claude Code" }).click();
  await page.getByRole("menuitemradio", { name: /Codex/ }).click();
  await userEvent.keyboard("{Escape}");
  expect(
    commands().findLast((c) => c.path === "/api/quick-starts/options")?.body,
  ).toEqual({ repositoryId: "repo1", configId: "cfgcodex" });
  await expect
    .element(toolbar.getByRole("status"))
    .toHaveTextContent("正在读取配置…");
  await expect
    .element(toolbar.getByRole("button", { name: "Agent：Codex" }))
    .toBeDisabled();
  await input.fill("修复首页排序");
  await expect
    .element(page.getByRole("button", { name: "发送", exact: true }))
    .toBeDisabled();

  pending.resolve(Response.json([effort]));
  await expect
    .element(toolbar.getByRole("button", { name: "思考：中" }))
    .toBeEnabled();
  await expect
    .element(toolbar.getByRole("button", { name: "Agent：Codex" }))
    .toBeEnabled();
  await expect
    .element(page.getByRole("button", { name: "发送", exact: true }))
    .toBeEnabled();
});

test("a remembered agent that is gone falls back to the first hybrid config", async () => {
  localStorage.setItem(
    "webmux.quickStart",
    JSON.stringify({ repositoryId: "gone", configId: "cfgdeled" }),
  );
  const { toolbar, place } = await open();
  await expect
    .element(toolbar.getByRole("button", { name: "Agent：Claude Code" }))
    .toBeVisible();
  await expect
    .element(place.getByRole("button", { name: "仓库：webmux" }))
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

// The quick start's uploads go to the staging endpoint; record where.
class StagingXHR {
  static sent: { url: string; xhr: StagingXHR }[] = [];
  upload = { onprogress: null as ((event: ProgressEvent) => void) | null };
  status = 0;
  statusText = "";
  response: unknown;
  responseType = "";
  timeout = 0;
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onabort: (() => void) | null = null;
  ontimeout: (() => void) | null = null;
  url = "";
  open(_method: string, url: string) {
    this.url = url;
  }
  setRequestHeader() {}
  send() {
    StagingXHR.sent.push({ url: this.url, xhr: this });
  }
  abort() {
    this.onabort?.();
  }
}

test("files attached before the session exist go with the first message", async () => {
  StagingXHR.sent = [];
  vi.stubGlobal("XMLHttpRequest", StagingXHR);
  let step = "precheck";
  respond(/^POST \/api\/quick-starts$/, () =>
    Response.json({ error: "不行", step }, { status: 409 }),
  );
  const { input } = await open();
  const picker = page
    .getByLabelText("选择上传文件")
    .element() as HTMLInputElement;
  const transfer = new DataTransfer();
  transfer.items.add(
    new File(["# 规格"], "spec.md", { type: "text/markdown" }),
  );
  picker.files = transfer.files;
  picker.dispatchEvent(new Event("change", { bubbles: true }));
  await expect.poll(() => StagingXHR.sent.length).toBe(1);
  expect(StagingXHR.sent[0]!.url).toBe("/api/quick-starts/uploads");
  const { xhr } = StagingXHR.sent[0]!;
  xhr.status = 201;
  xhr.response = { path: "/tmp/staging/spec.md" };
  xhr.onload?.();
  const chips = page.getByRole("list", { name: "待发送附件" });
  await expect.element(chips).toHaveTextContent("spec.md");

  // A failure before the first message keeps the attachment for a retry.
  await input.fill("按附件修复");
  await page.getByRole("button", { name: "发送", exact: true }).click();
  await expect.element(page.getByRole("alert")).toBeVisible();
  expect(
    commands().findLast((c) => c.path === "/api/quick-starts")?.body,
  ).toMatchObject({ attachments: [{ path: "/tmp/staging/spec.md" }] });
  await expect.element(chips).toHaveTextContent("spec.md");

  // Once the first message failed, the files belong to the new session.
  step = "prompt";
  await page.getByRole("button", { name: "发送", exact: true }).click();
  await expect.element(chips).not.toBeInTheDocument();
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
