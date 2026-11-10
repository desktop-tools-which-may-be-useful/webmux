import { afterEach, beforeEach, expect, test } from "vitest";
import { page } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import RepositoryGroup from "../../src/components/RepositoryGroup.svelte";
import type { SessionInfo } from "../../src/schemas";
import { repository, session, worktree } from "./shell-fixture";

beforeEach(async () => {
  await page.viewport(1024, 900);
});

afterEach(async () => {
  await cleanup();
  document.body.style.padding = "";
});

function renderGroup(
  focused: boolean,
  linked: Parameters<typeof worktree>[0] = {},
  sessions: SessionInfo[] = [],
) {
  render(RepositoryGroup, {
    repo: repository({
      worktrees: [
        worktree({ id: "main" }),
        worktree({
          id: "linked",
          kind: "linked",
          branch: "wt/feature",
          ...linked,
        }),
      ],
    }),
    sessions,
    filtering: false,
    focused,
    open: true,
    onopenchange: () => {},
    now: Date.now(),
    onstart: () => {},
    ontogglepin: () => {},
    onremove: () => {},
    onclose: () => {},
  });
}

test("a focused repository lists its worktrees without repeating a heading", async () => {
  await page.viewport(430, 932);
  renderGroup(true);
  await expect
    .element(page.getByRole("region", { name: /^仓库 / }))
    .toBeVisible();
  await expect.element(page.getByText("main", { exact: true })).toBeVisible();
  expect(page.getByRole("heading", { level: 2 }).elements()).toEqual([]);
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );
});

test("an unfocused repository keeps its name as a foldable heading", async () => {
  renderGroup(false);
  const heading = page.getByRole("heading", { level: 2 });
  await expect.element(heading).toBeVisible();
  await expect
    .element(heading.getByRole("button", { expanded: true }))
    .toBeVisible();
});

// A worktree with a session is always on the home page.
const inLinked = session({
  id: "linked01",
  workspace: {
    repositoryId: "repo1",
    worktreeId: "linked",
    worktreePath: "/repos/webmux",
  },
});

test("a worktree ahead of its upstream says so in its title, without a behind count", async () => {
  renderGroup(false, { upstream: "origin/wt/feature", ahead: 3, behind: 0 }, [
    inLinked,
  ]);
  const linked = page.getByRole("region", { name: "wt/feature" });
  await expect
    .element(linked.getByRole("img", { name: "领先上游 3 个提交" }))
    .toHaveTextContent("3");
  expect(linked.getByRole("img", { name: /落后/ }).elements()).toEqual([]);
  // Synced and untracked worktrees show no indicator.
  expect(
    page.getByRole("region", { name: "main" }).getByRole("img").elements(),
  ).toEqual([]);
});

test("a deleted upstream is marked without counts", async () => {
  renderGroup(false, { upstream: "origin/wt/feature", upstreamGone: true }, [
    inLinked,
  ]);
  const linked = page.getByRole("region", { name: "wt/feature" });
  await expect.element(linked.getByText("上游已删除")).toBeVisible();
  expect(linked.getByRole("img").elements()).toEqual([]);
});
