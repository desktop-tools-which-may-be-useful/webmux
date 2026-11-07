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

function renderFocused() {
  render(RepositoryGroup, {
    repo: repository({
      worktrees: [
        worktree({ id: "main" }),
        worktree({ id: "linked", kind: "linked", branch: "wt/feature" }),
      ],
    }),
    sessions: [],
    filtering: false,
    focused: true,
    open: true,
    onopenchange: () => {},
    gitReady: true,
    registryReady: true,
    refreshing: false,
    now: Date.now(),
    onstart: () => {},
    onmanage: () => {},
    onunregister: () => {},
    ontogglepin: () => {},
    onremove: () => {},
    onclose: () => {},
  });
}

test("430px focused repository header stacks actions below a one-line title", async () => {
  await page.viewport(430, 932);
  // The page pads its content by 16px on phones.
  document.body.style.padding = "0 16px";
  renderFocused();

  const heading = page.getByRole("heading", { level: 2 });
  await expect.element(heading).toBeVisible();
  const manage = page.getByRole("button", { name: "管理工作树" });
  const create = page.getByRole("button", { name: "新建工作树" });
  await expect.element(manage).toBeVisible();
  await expect.element(create).toBeVisible();

  const title = heading.element().getBoundingClientRect();
  const actions = [
    manage.element().getBoundingClientRect(),
    create.element().getBoundingClientRect(),
  ];
  // The title keeps its line instead of shrinking to one Han character per row.
  expect(title.height).toBeLessThan(40);
  // The actions take their own row below the title.
  for (const action of actions) {
    expect(action.top).toBeGreaterThanOrEqual(title.bottom);
  }
  // The toolbar stays on one line and does not overflow the viewport.
  expect(Math.abs(actions[0].top - actions[1].top)).toBeLessThanOrEqual(1);
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );
});
