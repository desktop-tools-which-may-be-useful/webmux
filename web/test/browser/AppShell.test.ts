import { afterEach, beforeEach, expect, test } from "vitest";
import { page, userEvent } from "vitest/browser";
import { render } from "vitest-browser-svelte/pure";
import { resetAttentionToasts } from "../../src/attention-toast.svelte";
import { openSession, shellUi } from "../../src/shell.svelte";
import { createTerminalDrafts } from "../../src/terminal-draft.svelte";
import {
  commands,
  installChatFixture,
  respond,
  socketAt,
} from "./chat-fixture";
import ShellHarness from "./ShellHarness.svelte";
import {
  chat,
  launchConfig,
  record,
  repository,
  session,
  worktree,
} from "./shell-fixture";

installChatFixture();
beforeEach(() => {
  resetAttentionToasts();
  shellUi.setMode("workbench");
  shellUi.setCollapsed(false);
  history.replaceState(null, "", "/");
  respond(/^GET \/api\/repositories$/, () => Response.json([repository()]));
  respond(/^GET \/api\/launch-configs$/, () => Response.json([launchConfig()]));
  respond(/^GET \/api\/sessions$/, () => Response.json([record()]));
});
afterEach(() => {
  localStorage.removeItem("webmux.shellMode");
  localStorage.removeItem("webmux.sidebarCollapsed");
  localStorage.removeItem("webmux.toastNotifications");
  resetAttentionToasts();
  history.replaceState(null, "", "/");
});

const sidebar = () => page.getByRole("complementary", { name: "侧栏" });

async function snapshot(
  sessions = [chat({ id: "chataaaa", title: "修复排序" })],
) {
  const socket = await socketAt("/ws/sessions");
  socket.frame({ type: "snapshot", sessions, forwards: [] } as never);
  return socket;
}

// Must stay first: shellUi reads the saved width once, when it loads.
test("without a saved width the sidebar starts at its default 288px", async () => {
  expect(localStorage.getItem("webmux.sidebarWidth")).toBeNull();
  expect(shellUi.width).toBe(288);
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  await expect
    .poll(() => sidebar().element().getBoundingClientRect().width)
    .toBe(288);
});

test("the chat button takes a workbench home to quick start", async () => {
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  const chatMode = sidebar().getByRole("button", { name: "聊天模式" });
  await expect.element(chatMode).toHaveAttribute("aria-pressed", "false");
  await chatMode.click();
  expect(location.pathname).toBe("/new");
  expect(localStorage.getItem("webmux.shellMode")).toBe("chat");
  await expect.element(chatMode).toHaveAttribute("aria-pressed", "true");
  // Quick start puts the caret in its input.
  await expect
    .element(page.getByRole("textbox", { name: "任务描述" }))
    .toHaveFocus();
  await expect
    .element(sidebar().getByRole("list", { name: "聊天历史" }))
    .toBeVisible();
});

test("on a session page switching mode only swaps the sidebar", async () => {
  history.replaceState(null, "", "/s/shellaaa");
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot([session({ id: "shellaaa", title: "pnpm dev" })]);
  await expect
    .element(sidebar().getByRole("tree", { name: "仓库与会话" }))
    .toBeVisible();
  await sidebar().getByRole("button", { name: "聊天模式" }).click();
  expect(location.pathname).toBe("/s/shellaaa");
  await expect
    .element(sidebar().getByRole("button", { name: "聊天模式" }))
    .toHaveFocus();
  await expect.element(page.getByTestId("session-page")).toBeVisible();
  // A terminal session has no row among the chats; archived records fold below.
  await sidebar()
    .getByRole("button", { name: /已归档/ })
    .click();
  const archived = sidebar().getByRole("list", { name: "已归档聊天" });
  await expect.element(archived).toBeVisible();
  expect(
    archived
      .getByRole("button")
      .elements()
      .filter((row) => row.getAttribute("aria-current") === "page"),
  ).toEqual([]);
});

test("the collapsed rail is remembered and keeps the mode and primary action", async () => {
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  await page.getByRole("button", { name: "收起侧栏" }).click();
  expect(localStorage.getItem("webmux.sidebarCollapsed")).toBe("true");
  const rail = page.getByRole("navigation", { name: "窄栏导航" });
  await expect
    .element(rail.getByRole("button", { name: "展开侧栏" }))
    .toBeVisible();
  await expect
    .element(rail.getByRole("button", { name: "新建会话" }))
    .toBeVisible();
  expect(
    document.querySelector(".app-shell")?.getAttribute("data-collapsed"),
  ).toBe("true");
  await rail.getByRole("button", { name: "展开侧栏" }).click();
  expect(localStorage.getItem("webmux.sidebarCollapsed")).toBe("false");
});

test("the sidebar's new-session action opens the shared dialog", async () => {
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  await sidebar().getByRole("button", { name: "新建会话" }).click();
  await expect
    .element(page.getByRole("dialog", { name: "新建会话" }))
    .toBeVisible();
});

test("opening any session keeps the sidebar's mode", () => {
  for (const mode of ["workbench", "chat"] as const) {
    shellUi.setMode(mode);
    openSession(session({ id: "shellaaa" }));
    expect(shellUi.mode).toBe(mode);
    expect(location.pathname).toBe("/s/shellaaa");
    openSession(chat({ id: "chataaaa" }));
    expect(shellUi.mode).toBe(mode);
    expect(location.pathname).toBe("/s/chataaaa");
  }
});

test("renaming from the sidebar menu uses the shared dialog", async () => {
  shellUi.setMode("chat");
  history.replaceState(null, "", "/s/otherabc");
  respond(/^PATCH \/api\/sessions\/chataaaa$/, () =>
    Response.json(chat({ id: "chataaaa", title: "首页排序" })),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  const socket = await snapshot();
  const chats = sidebar().getByRole("list", { name: "聊天历史" });
  await sidebar()
    .getByRole("button", { name: /已归档/ })
    .click();
  const archived = sidebar().getByRole("list", { name: "已归档聊天" });
  // Archived records resume or delete their record; they have no rename.
  await archived.getByRole("button", { name: "会话操作：整理文档" }).click();
  await expect
    .element(page.getByRole("menuitem", { name: "删除记录" }))
    .toBeVisible();
  expect(page.getByRole("menuitem", { name: "修改标题" }).elements()).toEqual(
    [],
  );
  await userEvent.keyboard("{Escape}");
  await chats.getByRole("button", { name: "会话操作：修复排序" }).click();
  await page.getByRole("menuitem", { name: "修改标题" }).click();
  const dialog = page.getByRole("dialog");
  const field = dialog.getByRole("textbox");
  await expect.element(field).toHaveValue("修复排序");
  await field.fill("首页排序");
  await userEvent.keyboard("{Enter}");
  await expect.element(dialog).not.toBeInTheDocument();
  expect(commands().at(-1)).toMatchObject({
    method: "PATCH",
    path: "/api/sessions/chataaaa",
    body: { name: "首页排序" },
  });
  // The sidebar shows what the list event brings.
  socket.frame({
    type: "updated",
    session: chat({ id: "chataaaa", title: "首页排序" }),
  } as never);
  await expect
    .element(chats.getByRole("button", { name: "首页排序（空闲）" }))
    .toBeVisible();
});

test("a rejected name stays in the dialog and the row keeps its title", async () => {
  shellUi.setMode("chat");
  history.replaceState(null, "", "/s/otherabc");
  respond(/^PATCH \/api\/sessions\/chataaaa$/, () =>
    Response.json({ error: "名称不能包含控制字符" }, { status: 400 }),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  const chats = sidebar().getByRole("list", { name: "聊天历史" });
  await chats.getByRole("button", { name: "会话操作：修复排序" }).click();
  await page.getByRole("menuitem", { name: "修改标题" }).click();
  await page.getByRole("dialog").getByRole("textbox").fill("坏\u0007名");
  await page.getByRole("dialog").getByRole("button", { name: "保存" }).click();
  await expect
    .element(page.getByRole("dialog").getByText("名称不能包含控制字符"))
    .toBeVisible();
  await expect
    .element(chats.getByRole("button", { name: "修复排序（空闲）" }))
    .toBeVisible();
});

test("an archived chat resumes on its chat side, or says why it cannot", async () => {
  shellUi.setMode("chat");
  history.replaceState(null, "", "/s/otherabc");
  respond(/^POST \/api\/sessions\/archivea\/resume$/, () =>
    Response.json(
      { error: "工作目录 /repos/webmux 已不存在" },
      { status: 409 },
    ),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  await sidebar()
    .getByRole("button", { name: /已归档/ })
    .click();
  const archived = sidebar().getByRole("list", { name: "已归档聊天" });
  await archived.getByRole("button", { name: "整理文档（已结束）" }).click();
  await expect
    .element(sidebar().getByRole("alert"))
    .toMatchTextContent(
      /无法恢复“整理文档”：工作目录 \/repos\/webmux 已不存在/,
    );
  expect(location.pathname).toBe("/s/otherabc");
  expect(commands().at(-1)).toMatchObject({
    path: "/api/sessions/archivea/resume",
    body: { view: "chat" },
  });

  respond(/^POST \/api\/sessions\/archivea\/resume$/, () =>
    Response.json(chat({ id: "archivea", title: "整理文档" })),
  );
  await archived.getByRole("button", { name: "整理文档（已结束）" }).click();
  await expect.poll(() => location.pathname).toBe("/s/archivea");
});

test("the sidebar menu archives a live chat and deletes an archived record", async () => {
  shellUi.setMode("chat");
  history.replaceState(null, "", "/s/otherabc");
  respond(
    /^DELETE \/api\/sessions\/chataaaa$/,
    () => new Response(null, { status: 204 }),
  );
  respond(
    /^DELETE \/api\/sessions\/archivea$/,
    () => new Response(null, { status: 204 }),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  const chats = sidebar().getByRole("list", { name: "聊天历史" });
  await chats.getByRole("button", { name: "会话操作：修复排序" }).click();
  await page.getByRole("menuitem", { name: "停止会话并归档" }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "停止并归档", exact: true })
    .click();
  await expect
    .poll(() =>
      commands().some(
        (c) => c.method === "DELETE" && c.path === "/api/sessions/chataaaa",
      ),
    )
    .toBe(true);

  await sidebar()
    .getByRole("button", { name: /已归档/ })
    .click();
  const archived = sidebar().getByRole("list", { name: "已归档聊天" });
  await archived.getByRole("button", { name: "会话操作：整理文档" }).click();
  await page.getByRole("menuitem", { name: "删除记录" }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "确认" })
    .click();
  await expect
    .poll(() =>
      commands().some(
        (c) =>
          c.method === "DELETE" &&
          c.path === "/api/sessions/archivea" &&
          c.query === "?forget=1",
      ),
    )
    .toBe(true);
});

test("deleting an archived record whose linked worktree still exists opens worktree removal confirmation", async () => {
  shellUi.setMode("chat");
  history.replaceState(null, "", "/s/otherabc");
  const linked = worktree({
    id: "wt1",
    path: "/repos/webmux.worktrees/wt1",
    kind: "linked",
    branch: "wt/fix-sort",
  });
  respond(/^GET \/api\/repositories$/, () =>
    Response.json([repository({ worktrees: [worktree(), linked] })]),
  );
  respond(/^GET \/api\/sessions$/, () =>
    Response.json([
      record({
        id: "archivew",
        title: "分支聊天",
        cwd: linked.path,
        workspace: {
          repositoryId: "repo1",
          worktreeId: "wt1",
          worktreePath: linked.path,
        },
      }),
    ]),
  );
  respond(
    /^POST \/api\/repositories\/repo1\/worktrees\/wt1\/removal-preview$/,
    () =>
      Response.json({
        confirmationId: "conf1234",
        expiresAt: "2026-10-28T01:00:00Z",
        worktree: linked,
        sessions: [],
        archived: [
          record({
            id: "archivew",
            title: "分支聊天",
            cwd: linked.path,
            workspace: {
              repositoryId: "repo1",
              worktreeId: "wt1",
              worktreePath: linked.path,
            },
          }),
        ],
        blockers: [],
        forceAllowed: true,
      }),
  );
  respond(
    /^POST \/api\/repositories\/repo1\/worktrees\/wt1\/remove$/,
    () => new Response(null, { status: 204 }),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  await sidebar()
    .getByRole("button", { name: /已归档/ })
    .click();
  const archived = sidebar().getByRole("list", { name: "已归档聊天" });
  await archived.getByRole("button", { name: "会话操作：分支聊天" }).click();
  await page.getByRole("menuitem", { name: "删除记录" }).click();
  const dialog = page.getByRole("dialog", { name: "删除 worktree" });
  await expect.element(dialog).toBeVisible();
  await expect.element(dialog).toMatchTextContent(/wt\/fix-sort/);
  await dialog.getByRole("button", { name: "确认安全删除" }).click();
  await expect
    .poll(() =>
      commands().some(
        (c) =>
          c.method === "POST" &&
          c.path === "/api/repositories/repo1/worktrees/wt1/remove",
      ),
    )
    .toBe(true);
});

test("shows a top-right toast when a session raises attention and navigates on click", async () => {
  render(ShellHarness, { drafts: createTerminalDrafts() });
  const socket = await snapshot([
    chat({ id: "chataaaa", title: "修复排序" }),
    session({ id: "shellbbb", title: "数据库迁移" }),
  ]);

  socket.frame({
    type: "updated",
    session: session({
      id: "shellbbb",
      title: "数据库迁移",
      status: "waiting",
      attention: {
        kind: "permission",
        source: "agent",
        since: "2026-11-05T12:00:00Z",
      },
    }),
  } as never);

  const region = page.getByRole("region", { name: "会话通知" });
  await expect.element(region).toBeVisible();
  const openButton = region.getByRole("button", {
    name: "打开会话 数据库迁移：等待授权 · Agent 请求执行工具操作",
  });
  await expect.element(openButton).toBeVisible();
  await openButton.click();
  expect(location.pathname).toBe("/s/shellbbb");
  await expect.element(region).not.toBeInTheDocument();
});
