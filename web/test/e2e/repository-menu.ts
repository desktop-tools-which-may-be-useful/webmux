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
  const open = page.getByRole("button", { name: "打开侧栏" });
  const trigger = tree
    .getByRole("button", {
      name: new RegExp(`^仓库操作：${repositoryName ?? ""}`),
    })
    .first();
  // The sidebar renders after the page loads and a phone drawer may still be
  // sliding shut from an earlier step: retry from whichever form is shown.
  await expect(async () => {
    await expect(tree.or(open).first()).toBeVisible({ timeout: 2000 });
    if (!(await tree.isVisible())) await open.click({ timeout: 2000 });
    await trigger.click({ timeout: 2000 });
  }).toPass();
  await page.getByRole("menuitem", { name: item }).click();
}

/** Asserts the repository details dialog's "主页 n / 共 m" worktree count. */
export async function expectHomeWorktrees(
  page: Page,
  shown: number,
  total: number,
  repositoryName?: string,
) {
  // A closed phone drawer stays in the DOM, so look only at the open dialog.
  const dialog = page.locator('[role="dialog"][data-state="open"]');
  await expect(async () => {
    await repositoryMenu(page, "仓库详情…", repositoryName);
    try {
      await expect(dialog).toContainText(`主页 ${shown} / 共 ${total}`, {
        timeout: 2000,
      });
    } finally {
      await page.keyboard.press("Escape");
      await expect(dialog).toHaveCount(0);
    }
  }).toPass();
}
