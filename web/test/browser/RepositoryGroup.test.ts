import { afterEach, beforeEach, expect, test } from "vitest";
import { page } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import RepositoryGroup from "../../src/components/RepositoryGroup.svelte";
import { repository, worktree } from "./shell-fixture";

beforeEach(async () => {
  await page.viewport(1024, 900);
});

afterEach(async () => {
  await cleanup();
  document.body.style.padding = "";
});

function renderGroup(focused: boolean) {
  render(RepositoryGroup, {
    repo: repository({
      worktrees: [
        worktree({ id: "main" }),
        worktree({ id: "linked", kind: "linked", branch: "wt/feature" }),
      ],
    }),
    sessions: [],
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
