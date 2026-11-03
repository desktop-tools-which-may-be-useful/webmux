import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { suggestWorktreeBranch } from "../../src/branchName";
import type { Repository } from "../../src/schemas";
import Harness from "./WorktreeCreateFieldsHarness.svelte";

const head = "a".repeat(40);
const repository = {
  id: "repo-1",
  name: "project",
  commonDir: "/repos/project/.git",
  anchorPath: "/repos/project",
  availability: "available",
  error: "",
  checkedAt: "2026-10-14T00:00:00Z",
  worktrees: [
    {
      id: "main",
      repositoryId: "repo-1",
      path: "/repos/project",
      kind: "main",
      branch: "main",
      head,
      detached: false,
      locked: false,
      lockReason: "",
      prunable: false,
      available: true,
      dirty: false,
      hasUntracked: false,
      hasIgnored: false,
      error: "",
    },
    {
      id: "linked",
      repositoryId: "repo-1",
      path: "/repos/project.worktrees/calm-river",
      kind: "linked",
      branch: "feature/calm-river",
      head,
      detached: false,
      locked: false,
      lockReason: "",
      prunable: false,
      available: true,
      dirty: false,
      hasUntracked: false,
      hasIgnored: false,
      error: "",
    },
  ],
} satisfies Repository;

const branches = {
  branches: [
    { name: "main", head, worktreePaths: ["/repos/project"] },
    { name: "wt/taken-branch", head, worktreePaths: [] },
    {
      name: "feature/calm-river",
      head,
      worktreePaths: ["/repos/project.worktrees/calm-river"],
    },
  ],
  bases: [{ name: "refs/heads/main", head, subject: "修复终端重连" }],
  defaultBase: head,
  defaultBaseSubject: "修复终端重连",
  defaultBranch: "main",
  defaultParent: "/repos/project.worktrees",
};

type RefreshHandler = (
  branch: string,
  signal: AbortSignal | null,
) => Response | Promise<Response>;

function localRefresh(response: typeof branches, branch: string) {
  const selected = response.branches.find((item) => item.name === branch);
  return {
    branch,
    base: selected?.head || response.defaultBase || "",
    baseSubject:
      response.bases.find((item) => item.name === `refs/heads/${branch}`)
        ?.subject || response.defaultBaseSubject,
    baseSource: "local",
    upstream: "",
    fetchWarning: "该分支未配置上游，保留本地基线",
  };
}

function remoteRefresh(branch: string, base: string, subject = "remote") {
  return {
    branch,
    base,
    baseSubject: subject,
    baseSource: "remote",
    upstream: `origin/${branch}`,
    fetchWarning: "",
  };
}

function deferred<T>() {
  return Promise.withResolvers<T>();
}

function serve(status = 200, response = branches, onrefresh?: RefreshHandler) {
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const url = new URL(String(input), location.href);
      if (url.pathname === "/api/repositories/repo-1/branches/refresh") {
        const body = JSON.parse(String(init?.body)) as { branch: string };
        if (onrefresh) return onrefresh(body.branch, init?.signal ?? null);
        return Response.json(localRefresh(response, body.branch));
      }
      if (url.pathname !== "/api/repositories/repo-1/branches") {
        throw new Error(`Unexpected request: ${url.pathname}`);
      }
      if (status !== 200) {
        return Response.json({ error: "无法读取分支" }, { status });
      }
      return Response.json(response);
    }),
  );
}

const branchInput = () =>
  page.getByRole("textbox", { name: "新分支名称", exact: true });
const pathInput = () =>
  page.getByRole("textbox", { name: "新工作树完整路径", exact: true });
const ready = () => page.getByRole("status", { name: "表单就绪" });
const baseInput = () =>
  page.getByRole("combobox", { name: "起始提交", exact: true });
const startBranchInput = () =>
  page.getByRole("combobox", { name: "起始分支", exact: true });
const reference = () => page.getByRole("status", { name: "提交引用" });

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  }
});

test("shows commit titles while preserving exact references and manual input", async () => {
  serve(200, {
    ...branches,
    bases: [
      ...branches.bases,
      {
        name: "refs/heads/origin/main",
        head,
        subject: "修复终端重连",
      },
      {
        name: "refs/remotes/origin/main",
        head: "b".repeat(40),
        subject: "修复终端重连",
      },
    ],
  });
  render(Harness, { repository });
  const input = page.getByRole("combobox", { name: "起始提交", exact: true });
  const reference = page.getByRole("status", { name: "提交引用" });
  await expect.element(input).toHaveValue("修复终端重连（当前主工作区）");
  await expect.element(reference).toHaveTextContent(head);

  await input.fill("修复终端重连（远程：origin/main）");
  await expect.element(reference).toHaveTextContent("refs/remotes/origin/main");
  await input.fill("修复终端重连（本地：origin/main）");
  await expect.element(reference).toHaveTextContent("refs/heads/origin/main");
  await input.fill("修复终端重连（本地：main）");
  await expect.element(reference).toHaveTextContent("refs/heads/main");

  for (const value of ["release/my-task", "c".repeat(40)]) {
    await input.fill(value);
    await expect.element(reference).toHaveTextContent(value);
    await expect.element(ready()).toHaveTextContent("ready");
  }
  await input.fill("");
  await expect.element(reference).toHaveTextContent("");
  await expect.element(ready()).toHaveTextContent("not-ready");
});

test("shows a detached default title without branch candidates", async () => {
  serve(200, {
    ...branches,
    bases: [],
    defaultBaseSubject: "独立提交 <title> & 引号",
    defaultBranch: null,
  });
  render(Harness, { repository });
  await expect
    .element(page.getByRole("combobox", { name: "起始提交", exact: true }))
    .toHaveValue("独立提交 <title> & 引号（当前主工作区）");
  await expect
    .element(page.getByRole("status", { name: "提交引用" }))
    .toHaveTextContent(head);
  await expect.element(ready()).toHaveTextContent("ready");
});

test("prefills an editable suggestion and does not refill after a manual clear", async () => {
  vi.spyOn(Math, "random").mockReturnValue(0);
  serve();
  const expected = suggestWorktreeBranch(
    {
      branchNames: branches.branches.map((item) => item.name),
      worktreePaths: repository.worktrees.map((tree) => tree.path),
    },
    { random: () => 0 },
  );
  render(Harness, { repository });

  await expect.element(branchInput()).toHaveValue(expected);
  await expect
    .element(pathInput())
    .toHaveValue(`/repos/project.worktrees/${expected.split("/").at(-1)}`);
  await expect.element(ready()).toHaveTextContent("ready");
  expect(expected.split("/").at(-1)).not.toBe("calm-river");

  await branchInput().fill("feature/my-task");
  await expect.element(branchInput()).toHaveValue("feature/my-task");
  await expect
    .element(pathInput())
    .toHaveValue("/repos/project.worktrees/my-task");

  await branchInput().fill("");
  await expect.element(branchInput()).toHaveValue("");
  await expect.element(ready()).toHaveTextContent("not-ready");

  await page.getByRole("button", { name: "换一个", exact: true }).click();
  await expect.element(branchInput()).toHaveValue(expected);
  await expect.element(ready()).toHaveTextContent("ready");
  await page.getByRole("button", { name: "换一个", exact: true }).click();
  await expect.element(branchInput()).not.toHaveValue(expected);
  await expect
    .poll(() => branchInput().element().value)
    .toMatch(/^wt\/[a-z]+-[a-z]+$/);
});

test("mode switch regenerates a new branch and a failed list stays empty", async () => {
  vi.spyOn(Math, "random").mockReturnValue(0);
  serve();
  render(Harness, { repository });
  await expect
    .poll(() => branchInput().element().value)
    .toMatch(/^wt\/[a-z]+-[a-z]+$/);

  await page.getByRole("button", { name: "使用已有分支", exact: true }).click();
  await expect.element(ready()).toHaveTextContent("not-ready");
  await page.getByRole("button", { name: "创建新分支", exact: true }).click();
  await expect
    .poll(() => branchInput().element().value)
    .toMatch(/^wt\/[a-z]+-[a-z]+$/);

  await cleanup();
  serve(503);
  render(Harness, { repository });
  await expect
    .element(page.getByRole("alert"))
    .toHaveTextContent("无法读取分支");
  await expect.element(branchInput()).toHaveValue("");
  await expect
    .element(page.getByRole("button", { name: "换一个", exact: true }))
    .toBeDisabled();
});

test("search selects the literal branch and ignores occupied matches without losing the choice", async () => {
  serve(200, {
    ...branches,
    branches: [
      ...branches.branches,
      { name: "feature/branch-024", head, worktreePaths: [] },
      { name: "feature/branch-042", head, worktreePaths: [] },
    ],
  });
  render(Harness, { repository });
  await page.getByRole("button", { name: "使用已有分支", exact: true }).click();
  const search = page.getByRole("combobox", {
    name: "已有本地分支",
    exact: true,
  });
  await expect.element(search).toBeEnabled();
  await search.fill("BRANCH-042");
  await expect
    .element(
      page.getByRole("option", { name: "feature/branch-042", exact: true }),
    )
    .toBeVisible();
  await expect
    .element(
      page.getByRole("option", { name: "feature/branch-024", exact: true }),
    )
    .not.toBeInTheDocument();
  await expect.element(ready()).toHaveTextContent("not-ready");
  await userEvent.keyboard("{Enter}");
  await expect
    .element(pathInput())
    .toHaveValue("/repos/project.worktrees/branch-042");
  await expect.element(ready()).toHaveTextContent("ready");

  await search.fill("main");
  await expect
    .element(page.getByRole("option", { name: /main/ }))
    .toHaveAttribute("aria-disabled", "true");
  await userEvent.keyboard("{Enter}");
  await expect
    .element(pathInput())
    .toHaveValue("/repos/project.worktrees/branch-042");
  await search.fill("no-such-branch");
  await userEvent.keyboard("{Enter}");
  await expect.element(ready()).toHaveTextContent("ready");
  await expect
    .element(pathInput())
    .toHaveValue("/repos/project.worktrees/branch-042");
});

test("upstream refresh stays nonblocking and manual refresh updates the default", async () => {
  const first = deferred<Response>();
  const remoteHead = "b".repeat(40);
  const manualHead = "c".repeat(40);
  let calls = 0;
  serve(200, branches, (branch) => {
    calls += 1;
    return calls === 1
      ? first.promise
      : Response.json(remoteRefresh(branch, manualHead, "手动刷新"));
  });
  render(Harness, { repository });

  await expect
    .element(page.getByRole("status", { name: "上游刷新状态" }))
    .toMatchTextContent(/正在获取 main/);
  await expect.element(reference()).toHaveTextContent(head);
  await expect.element(ready()).toHaveTextContent("ready");
  first.resolve(Response.json(remoteRefresh("main", remoteHead)));
  await expect.element(reference()).toHaveTextContent(remoteHead);
  await expect
    .element(page.getByRole("status", { name: "上游刷新状态" }))
    .toMatchTextContent(/远程最新 · origin\/main/);

  await page.getByRole("button", { name: "刷新上游", exact: true }).click();
  await expect.element(reference()).toHaveTextContent(manualHead);
  expect(calls).toBe(2);
});

test("a late response cannot replace a newer branch selection", async () => {
  const main = deferred<Response>();
  const selectedHead = "b".repeat(40);
  let mainSignal: AbortSignal | null = null;
  serve(200, branches, (branch, signal) => {
    if (branch === "main") {
      mainSignal = signal;
      return main.promise;
    }
    return Response.json(remoteRefresh(branch, selectedHead));
  });
  render(Harness, { repository });
  await expect
    .element(page.getByRole("status", { name: "上游刷新状态" }))
    .toMatchTextContent(/正在获取 main/);

  await startBranchInput().selectOptions("wt/taken-branch");
  await expect.element(reference()).toHaveTextContent(selectedHead);
  expect(mainSignal?.aborted).toBe(true);
  const lateResponse = Response.json(remoteRefresh("main", "c".repeat(40)));
  const lateRead = vi.spyOn(lateResponse, "json");
  main.resolve(lateResponse);
  await vi.waitFor(() => expect(lateRead).toHaveBeenCalled());
  await Promise.resolve();
  await expect.element(reference()).toHaveTextContent(selectedHead);
  await expect.element(startBranchInput()).toHaveValue("wt/taken-branch");
});

test("a refresh response preserves manual base edits made after the request", async () => {
  const pending = deferred<Response>();
  serve(200, branches, () => pending.promise);
  render(Harness, { repository });
  await expect
    .element(page.getByRole("status", { name: "上游刷新状态" }))
    .toMatchTextContent(/正在获取 main/);
  const manual = "d".repeat(40);
  await baseInput().fill(manual);
  const response = Response.json(remoteRefresh("main", "b".repeat(40)));
  const responseRead = vi.spyOn(response, "json");
  pending.resolve(response);
  await vi.waitFor(() => expect(responseRead).toHaveBeenCalled());
  await Promise.resolve();
  await expect.element(reference()).toHaveTextContent(manual);
  await expect.element(baseInput()).toHaveValue(manual);
});

test("a failed manual refresh clears the previous remote-latest label", async () => {
  const remoteHead = "b".repeat(40);
  let calls = 0;
  serve(200, branches, (branch) => {
    calls += 1;
    return calls === 1
      ? Response.json(remoteRefresh(branch, remoteHead))
      : Response.json({ error: "fetch failed" }, { status: 500 });
  });
  render(Harness, { repository });
  await expect
    .element(page.getByRole("status", { name: "上游刷新状态" }))
    .toMatchTextContent(/远程最新/);
  await page.getByRole("button", { name: "刷新上游", exact: true }).click();
  await expect
    .element(page.getByRole("status", { name: "上游刷新警告" }))
    .toMatchTextContent(/保留当前基线/);
  await expect
    .element(page.getByRole("status", { name: "上游刷新状态" }))
    .not.toBeInTheDocument();
  await expect.element(reference()).toHaveTextContent(remoteHead);
});

test("failed refreshes keep the local base and form availability", async () => {
  serve(200, branches, () =>
    Response.json({ error: "fetch failed" }, { status: 500 }),
  );
  render(Harness, { repository });
  await expect
    .element(page.getByRole("status", { name: "上游刷新警告" }))
    .toMatchTextContent(/保留当前基线/);
  await expect.element(reference()).toHaveTextContent(head);
  await expect.element(ready()).toHaveTextContent("ready");
});

test("mode switches and unmount abort pending refreshes", async () => {
  const pending = deferred<Response>();
  const signals: Array<AbortSignal | null> = [];
  serve(200, branches, (_branch, signal) => {
    signals.push(signal);
    return pending.promise;
  });
  render(Harness, { repository });
  await expect
    .element(page.getByRole("status", { name: "上游刷新状态" }))
    .toMatchTextContent(/正在获取 main/);
  await page.getByRole("button", { name: "使用已有分支", exact: true }).click();
  expect(signals).toHaveLength(1);
  expect(signals[0]?.aborted).toBe(true);

  await page.getByRole("button", { name: "创建新分支", exact: true }).click();
  await expect
    .element(page.getByRole("status", { name: "上游刷新状态" }))
    .toMatchTextContent(/正在获取 main/);
  expect(signals).toHaveLength(2);
  await cleanup();
  expect(signals[1]?.aborted).toBe(true);
});
