import { execFileSync } from "node:child_process";
import { writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { Locator, Page } from "@playwright/test";
import { test, expect, createShell, viewSwitch } from "./fixtures";

function git(root: string, ...args: string[]) {
  execFileSync("git", ["-C", root, ...args], {
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: "E2E",
      GIT_AUTHOR_EMAIL: "e2e@example.com",
      GIT_COMMITTER_NAME: "E2E",
      GIT_COMMITTER_EMAIL: "e2e@example.com",
    },
  });
}

async function modifiedRepository(workDir: string) {
  git(workDir, "init", "-q");
  const original =
    Array.from({ length: 200 }, (_, i) => `ordinary line ${i + 1}`).join("\n") +
    "\n";
  await writeFile(join(workDir, "code.txt"), original);
  git(workDir, "add", ".");
  git(workDir, "commit", "-qm", "initial");
  await writeFile(
    join(workDir, "code.txt"),
    original.replace("ordinary line 50\n", "changed line 50\n"),
  );
}

const header = (page: Page) => page.locator(".terminal-header");

async function expectOneRow(row: Locator) {
  const box = (await row.boundingBox())!;
  for (const child of await row.locator(":scope > :visible").all()) {
    const bounds = (await child.boundingBox())!;
    expect(bounds.y).toBeGreaterThanOrEqual(box.y - 0.5);
    expect(bounds.y + bounds.height).toBeLessThanOrEqual(
      box.y + box.height + 0.5,
    );
  }
  expect(
    await row.evaluate((element) => element.scrollWidth - element.clientWidth),
  ).toBeLessThanOrEqual(0);
}

async function openDiff(page: Page) {
  // The navigator starts open at every width.
  await page.getByRole("tab", { name: "修改", exact: true }).click();
  await page
    .getByRole("button", { name: "修改 code.txt", exact: true })
    .first()
    .click();
  await expect(
    page.locator(".code-diff-reader:visible [data-code]").first(),
  ).toContainText("changed line 50");
}

test("narrow headers keep one row and move rare actions into the menu", async ({
  page,
  workDir,
}, testInfo) => {
  await page.setViewportSize({ width: 393, height: 851 });
  await createShell(page, "demo-agent", workDir);
  const title = header(page).getByText("demo-agent", { exact: true });
  for (const width of [393, 320]) {
    await page.setViewportSize({ width, height: 851 });
    for (const view of ["代码", "终端"]) {
      await expectOneRow(header(page));
      await expect(title).toBeVisible();
      // 393px fits the whole title; 320px may truncate it but keeps it shown.
      if (width === 393)
        expect(
          await title.evaluate((e) => e.scrollWidth <= e.clientWidth),
        ).toBe(true);
      for (const name of ["全屏", "上传文件", "滚动到最新位置"])
        await expect(
          header(page).getByRole("button", { name, exact: true }),
        ).toBeHidden();
      // The phone header keeps back, title, the view switch and the menu.
      await expect(
        header(page).getByRole("button", { name: /^草稿模式/ }),
      ).toBeHidden();
      // An ordinary session's switch is the same tab control a hybrid one
      // uses, with its terminal and code tabs.
      const views = header(page).getByRole("tablist", { name: "会话视图" });
      await expect(views.getByRole("tab")).toHaveText(["终端", "代码"]);
      await expect(
        views.getByRole("tab", { name: "终端", exact: true }),
      ).toHaveAttribute("aria-selected", view === "代码" ? "true" : "false");
      await expect(
        header(page).getByRole("button", { name: "返回会话列表" }),
      ).toBeVisible();
      await views.getByRole("tab", { name: view, exact: true }).click();
    }
  }
  // Soft keys exist on touch devices; a narrow desktop window has the menu.
  if (testInfo.project.use.isMobile) {
    const draft = page
      .getByRole("complementary", { name: "终端软键" })
      .getByRole("button", { name: /^草稿模式/ });
    await draft.click();
    await expect(draft).toHaveAttribute("aria-pressed", "true");
    await draft.click();
    await expect(draft).toHaveAttribute("aria-pressed", "false");
  }
  await header(page)
    .getByRole("button", { name: "终端菜单", exact: true })
    .click();
  // Draft mode left the phone header for the soft keys and this menu.
  for (const name of ["全屏", "上传文件"])
    await expect(
      page.getByRole("menuitem", { name, exact: true }),
    ).toBeEnabled();
  await expect(
    page.getByRole("menuitem", { name: "滚动到最新位置", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("menuitemcheckbox", { name: /^草稿模式/ }),
  ).toBeVisible();
  // Splitting is rare on a phone: no header button, a "窗格" group instead.
  await expect(
    header(page).getByRole("button", { name: "分屏", exact: true }),
  ).toBeHidden();
  await expect(
    page.getByRole("menuitem", { name: "新建会话…", exact: true }),
  ).toBeVisible();
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("menuitem", { name: "上传文件", exact: true }).click();
  await chooser;
});

test("a phone Diff leaves most of the screen to code and reaches document actions from the code menu", async ({
  page,
  workDir,
}) => {
  await modifiedRepository(workDir);
  await page.setViewportSize({ width: 393, height: 851 });
  await createShell(page, "Phone diff", workDir);
  await viewSwitch(header(page), "代码").click();
  await openDiff(page);
  const tabs = page.getByRole("tablist", { name: "文件标签" });
  await expectOneRow(tabs.locator(".."));
  await expect(
    page.getByRole("button", { name: "完整文件", exact: true }),
  ).toBeHidden();
  await expect(
    page.getByRole("button", { name: "在独立窗口中显示代码" }),
  ).toBeHidden();
  const code = (await page
    .locator(".code-diff-reader")
    .filter({ visible: true })
    .boundingBox())!;
  expect(code.height).toBeGreaterThanOrEqual(851 * 0.7);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth - innerWidth,
    ),
  ).toBeLessThanOrEqual(0);

  await page.getByRole("button", { name: "代码菜单", exact: true }).click();
  const menu = page.getByRole("menu");
  await expect(menu).toContainText(join(workDir, "code.txt"));
  const box = (await menu.boundingBox())!;
  expect(box.y).toBeGreaterThanOrEqual(0);
  expect(box.y + box.height).toBeLessThanOrEqual(851);
  await page.getByRole("menuitem", { name: /修改前的文件/ }).click();
  await expect(
    page.getByRole("tab", { name: /^code\.txt · index(?: ●)?$/ }),
  ).toHaveAttribute("aria-selected", "true");
  await page.getByRole("button", { name: "跳转行", exact: true }).click();
  await page.locator("[data-code-goto-line]:visible input").fill("50");
  await page.locator("[data-code-goto-line]:visible input").press("Enter");
  await expect(
    page.locator("[data-code-reader]:visible [data-content]"),
  ).toContainText("ordinary line 50");
  // The Diff tab and its reading state stay open beside the snapshot.
  await expect(
    page.getByRole("tab", { name: /^code\.txt · unstaged(?: ●)?$/ }),
  ).toBeVisible();
});

test("wide headers keep every action and the code workspace stays on single rows", async ({
  page,
  workDir,
}, info) => {
  test.skip(info.project.use.isMobile, "Wide desktop layout");
  await modifiedRepository(workDir);
  await page.setViewportSize({ width: 1440, height: 900 });
  await createShell(page, "Wide layout", workDir);
  for (const name of ["全屏", "上传文件", "滚动到最新位置", "代码"])
    await expect(
      header(page).getByRole("button", { name, exact: true }),
    ).toBeVisible();
  await expect(
    header(page).getByRole("button", { name: /^草稿模式/ }),
  ).toBeVisible();
  await viewSwitch(header(page), "代码").click();
  await openDiff(page);
  const panel = page.getByRole("region", { name: "代码工作区" });
  await expect(panel.getByRole("navigation")).toContainText(workDir);
  const tabs = page.getByRole("tablist", { name: "文件标签" });
  const center = async (locator: Locator) => {
    const box = (await locator.boundingBox())!;
    return box.y + box.height / 2;
  };
  const tabRow = await center(tabs);
  expect(
    Math.abs(
      (await center(page.getByRole("button", { name: "刷新全部" }))) - tabRow,
    ),
  ).toBeLessThan(2);
  const path = page
    .getByRole("tabpanel")
    .filter({ visible: true })
    .getByTitle(join(workDir, "code.txt"), { exact: true });
  const fullFile = page.getByRole("button", { name: "完整文件", exact: true });
  expect(
    Math.abs((await center(path)) - (await center(fullFile))),
  ).toBeLessThan(2);
  await expect(page.getByRole("button", { name: "代码菜单" })).toBeHidden();
});
