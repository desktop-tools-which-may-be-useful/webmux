import { afterEach, expect, test, vi } from "vitest";
import { page } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import WorkspaceMenu from "../../src/components/WorkspaceMenu.svelte";

afterEach(async () => {
  await cleanup();
});

function open(occupied: boolean, pinned = true) {
  const ontogglepin = vi.fn();
  render(WorkspaceMenu, {
    repository: "project",
    onnewworktree: vi.fn(),
    onhome: vi.fn(),
    onmanage: vi.fn(),
    onsettings: vi.fn(),
    ondetails: vi.fn(),
    worktree: {
      title: "feature",
      ondetails: vi.fn(),
      pinned,
      occupied,
      ontogglepin,
      onremove: vi.fn(),
    },
  });
  return ontogglepin;
}

test("a pinned worktree holding sessions cannot be removed from the home page, and says why", async () => {
  const ontogglepin = open(true);
  await page.getByRole("button", { name: "仓库菜单：project" }).click();
  const item = page.getByRole("menuitem", { name: "从主页移除", exact: true });
  await expect.element(item).toHaveAttribute("aria-disabled", "true");
  await expect
    .element(page.getByText("有会话或归档记录，结束后才能移除"))
    .toBeVisible();
  await item.click({ force: true });
  expect(ontogglepin).not.toHaveBeenCalled();
});

test("an idle pinned worktree can be removed from the home page", async () => {
  const ontogglepin = open(false);
  await page.getByRole("button", { name: "仓库菜单：project" }).click();
  await page.getByRole("menuitem", { name: "从主页移除", exact: true }).click();
  expect(ontogglepin).toHaveBeenCalledOnce();
});

test("an unpinned worktree holding sessions can still be added to the home page", async () => {
  const ontogglepin = open(true, false);
  await page.getByRole("button", { name: "仓库菜单：project" }).click();
  await page.getByRole("menuitem", { name: "添加到主页" }).click();
  expect(ontogglepin).toHaveBeenCalledOnce();
});
