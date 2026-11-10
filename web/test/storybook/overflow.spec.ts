import { expect, test } from "@playwright/test";
import { assertRequiredButtons } from "./controls";

import { stories } from "./cases";
import { checkEachLayout, expectNoOverflow } from "./layouts";

test("worktree heading keeps its width above mobile actions", async ({
  page,
}) => {
  await page.goto(
    "/iframe.html?id=workspace-flat-layout--worktree-actions&viewMode=story&globals=theme:light",
  );
  const heading = page.getByRole("heading", {
    name: "codex/flatten-workspace-session-layout-for-mobile",
  });
  const start = page.getByRole("button", { name: "新建会话", exact: true });
  const menu = page.getByRole("button", { name: /^工作树菜单 / });

  await checkEachLayout(page, async ({ name }) => {
    await expect.soft(heading, name).toBeVisible();
    await expect.soft(start, name).toBeVisible();
    await expect.soft(menu, name).toBeVisible();

    const layout = await heading.evaluate((element) => {
      const title = element.parentElement!.getBoundingClientRect();
      const group = element.closest("section")!.getBoundingClientRect();
      const actions =
        element.parentElement!.nextElementSibling!.getBoundingClientRect();
      return {
        title: { width: title.width, top: title.top, bottom: title.bottom },
        groupWidth: group.width,
        actions: { top: actions.top, bottom: actions.bottom },
        mobile: !matchMedia("(min-width: 40rem)").matches,
      };
    });
    if (layout.mobile) {
      expect.soft(layout.title.width, name).toBeCloseTo(layout.groupWidth, 0);
      expect
        .soft(layout.actions.top, name)
        .toBeGreaterThanOrEqual(layout.title.bottom);
    } else {
      expect.soft(layout.actions.top, name).toBeLessThan(layout.title.bottom);
      expect
        .soft(layout.actions.bottom, name)
        .toBeGreaterThan(layout.title.top);
    }
    await expectNoOverflow(page, name);
    await menu.click();
    await expect
      .soft(page.getByRole("menuitem", { name: "从主页移除" }), name)
      .toBeVisible();
    await page.keyboard.press("Escape");
  });
});

for (const story of stories) {
  test(`${story} fits and keeps its controls reachable`, async ({ page }) => {
    const failures: string[] = [];
    page.on("pageerror", (error) => failures.push(error.message));
    await page.route("**/api/**", (route) => route.abort());
    await page.goto(
      `/iframe.html?id=ui-overflow--${story}&viewMode=story&globals=theme:light`,
    );
    await expect(page.getByTestId("overflow-story")).toBeVisible();

    await checkEachLayout(page, async ({ name, theme }) => {
      await expect
        .soft(page.locator("html"), name)
        .toHaveClass(theme === "dark" ? /dark/ : /^(?!.*\bdark\b)/);
      if (story === "quick-command-error") {
        await page.getByRole("button", { name: "保存", exact: true }).click();
        await expect.soft(page.getByRole("alert"), name).toBeVisible();
      }
      await page.evaluate(async () => {
        await Promise.all(
          document
            .getAnimations()
            .filter(
              (animation) =>
                animation.effect?.getComputedTiming().endTime !== Infinity,
            )
            .map((animation) => animation.finished.catch(() => {})),
        );
      });
      await expectNoOverflow(page, name);

      const dialogs = page.locator('[role="dialog"], [role="alertdialog"]');
      if (await dialogs.count()) {
        const dialog = dialogs.first();
        const bounds = await dialog.boundingBox();
        expect.soft(bounds, name).not.toBeNull();
        expect.soft(bounds!.y, name).toBeGreaterThanOrEqual(-1);
        expect
          .soft(bounds!.y + bounds!.height, name)
          .toBeLessThanOrEqual(page.viewportSize()!.height + 1);
      }
      const controls = (await dialogs.count())
        ? dialogs.first()
        : page.getByTestId("overflow-story");
      await assertRequiredButtons(controls, story);
      for (const button of await controls.getByRole("button").all()) {
        await expect.soft(button, name).toBeVisible();
        // Trial clicks already scroll and check stability. Keep an explicit
        // scroll for disabled controls, then assert full visibility in both cases.
        if (await button.isEnabled()) await button.click({ trial: true });
        else await button.scrollIntoViewIfNeeded();
        // Chromium can round the scroll offset by a fraction of a pixel.
        await expect.soft(button, name).toBeInViewport({ ratio: 0.99 });
      }
      if (story === "workspace-dialog") {
        const consent = page.getByRole("checkbox", {
          name: "已检查工作区路径",
        });
        await consent.check();
        await expect.soft(consent, name).toBeChecked();
        await consent.uncheck();
      }
      if (story === "session-row")
        await expect
          .soft(page.getByRole("link"), name)
          .toHaveAccessibleName(/unbroken_identifier/);
      if (story === "code-changes")
        await expect
          .soft(page.getByRole("button").first(), name)
          .toHaveAttribute("title", /unbroken_identifier/);
      expect.soft(failures, name).toEqual([]);
    });
  });
}

// The repository panel's sections and the worktree sync summary, with names
// and URLs long enough to wrap; a worktree's details open in the same
// dialog (drawer on phones) instead of a second one.
for (const story of [
  "overview",
  "worktrees",
  "branches",
  "remotes",
  "worktree-sync",
]) {
  test(`repository panel ${story} fits every layout`, async ({ page }) => {
    // Twelve layouts, and the worktrees story also opens a subview in each.
    test.slow();
    const failures: string[] = [];
    page.on("pageerror", (error) => failures.push(error.message));
    await page.route("**/api/**", (route) => route.abort());
    await page.goto(
      `/iframe.html?id=workspace-repository-panel--${story}&viewMode=story&globals=theme:light`,
    );
    await expect(page.getByTestId("repository-story")).toBeVisible();
    const dialog = page.getByRole("dialog");

    await checkEachLayout(page, async ({ name }) => {
      if (story === "branches")
        await expect
          .soft(page.getByRole("region", { name: "本地分支" }), name)
          .toBeVisible();
      if (story === "remotes")
        await expect
          .soft(page.getByRole("list", { name: "远程列表" }), name)
          .toBeVisible();
      await expectNoOverflow(page, name);
      if (story !== "worktree-sync") {
        // The dialog's height follows dvh, which can settle a frame after a
        // viewport change on a loaded CI runner.
        const viewport = page.viewportSize()!.height;
        await expect.soft
          .poll(
            async () => {
              const bounds = await dialog.boundingBox();
              return (
                bounds !== null &&
                bounds.y >= -1 &&
                bounds.y + bounds.height <= viewport + 1
              );
            },
            { message: name, timeout: 2_000 },
          )
          .toBe(true);
      }
      if (story === "worktrees") {
        await dialog
          .getByRole("list", { name: "仓库工作树列表" })
          .getByRole("button", { name: /^feature\/ahead-/ })
          .click();
        const back = dialog.getByRole("button", { name: "返回" });
        await expect.soft(back, name).toBeVisible();
        await expect.soft(dialog, name).toHaveCount(1);
        await expectNoOverflow(page, name);
        await back.click();
      }
      expect.soft(failures, name).toEqual([]);
    });
  });
}

test("keeps a full scratch list and its composer reachable", async ({
  page,
}) => {
  await page.goto(
    "/iframe.html?id=workspace-scratch-notes--full&viewMode=story&globals=theme:light",
  );
  const dialog = page.getByRole("dialog", { name: "暂存", exact: true });
  await expect(dialog).toBeVisible();

  await checkEachLayout(page, async ({ name }) => {
    await expectNoOverflow(page, name);
    const add = page.getByRole("button", { name: "添加暂存", exact: true });
    await add.scrollIntoViewIfNeeded();
    await expect.soft(add, name).toBeInViewport();
    await expect.soft(add, name).toBeDisabled();
    await expect.soft(page.getByText(/已达 20 条上限/), name).toBeVisible();
    const last = page.getByRole("textbox", {
      name: "暂存第 20 条",
      exact: true,
    });
    await last.scrollIntoViewIfNeeded();
    await expect.soft(last, name).toBeInViewport();
    await expect.soft(add, name).toBeInViewport();
    await page
      .getByRole("button", { name: "暂存第 20 条的更多操作", exact: true })
      .click();
    await expect
      .soft(
        page.getByRole("menuitem", { name: "下移暂存第 20 条", exact: true }),
        name,
      )
      .toBeDisabled();
    await page
      .getByRole("menuitem", { name: "删除暂存第 20 条", exact: true })
      .click();
    await page
      .getByRole("textbox", { name: "新建暂存", exact: true })
      .fill("稍后检查移动端");
    await expect.soft(add, name).toBeEnabled();
    await add.click();
    await expect.soft(last, name).toHaveValue("稍后检查移动端");
    await expectNoOverflow(page, name);
  });
});
