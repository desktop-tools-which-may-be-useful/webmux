import { expect, type Page } from "@playwright/test";

/**
 * Picks an item of a repository's "⋯" menu in the sidebar tree (the phone
 * drawer on narrow screens), the one place that holds repository actions.
 */
export async function repositoryMenu(
  page: Page,
  item: string,
  repositoryName?: string,
) {
  const tree = page.getByRole("tree", { name: "仓库与会话" });
  if (!(await tree.isVisible()))
    await page.getByRole("button", { name: "打开侧栏" }).click();
  await tree
    .getByRole("button", {
      name: new RegExp(`^仓库操作：${repositoryName ?? ""}`),
    })
    .first()
    .click();
  await page.getByRole("menuitem", { name: item }).click();
}

/** Asserts the repository details dialog's "主页 n / 共 m" worktree count. */
export async function expectHomeWorktrees(
  page: Page,
  shown: number,
  total: number,
  repositoryName?: string,
) {
  await expect(async () => {
    await repositoryMenu(page, "仓库详情…", repositoryName);
    try {
      await expect(page.getByRole("dialog")).toContainText(
        `主页 ${shown} / 共 ${total}`,
        { timeout: 2000 },
      );
    } finally {
      await page.keyboard.press("Escape");
      await expect(page.getByRole("dialog")).toHaveCount(0);
    }
  }).toPass();
}
