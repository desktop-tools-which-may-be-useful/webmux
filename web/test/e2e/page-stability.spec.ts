import { test, expect, revealSidebar } from "./fixtures";
import { settled } from "../performance/performance";

// Published UI evidence uses an isolated fixture identity.
test.beforeEach(async ({ page }) => {
  await page.route("**/api/config", async (route) => {
    const response = await route.fetch();
    const config = await response.json();
    await route.fulfill({
      response,
      json: { ...config, hostname: "demo-host" },
    });
  });
});

test("workspace defers settings and management forms until requested", async ({
  page,
}) => {
  const scripts: string[] = [];
  page.on("request", (request) => {
    if (request.resourceType() === "script") scripts.push(request.url());
  });
  await page.goto("/");
  await expect(page.getByText("还没有会话", { exact: true })).toBeVisible();
  expect(
    scripts.filter((url) =>
      /\/(Settings|NewSessionDialog|AddRepositoryDialog|RemoveWorktreeDialog)-/.test(
        url,
      ),
    ),
  ).toEqual([]);
  await page
    .getByRole("main")
    .getByRole("button", { name: "＋ 新建会话", exact: true })
    .first()
    .click();
  await expect(page.getByRole("dialog")).toBeVisible();
  expect(scripts.some((url) => /\/NewSessionDialog-/.test(url))).toBe(true);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeHidden();
  await revealSidebar(page);
  await page.getByRole("button", { name: "添加仓库", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "添加仓库" })).toBeVisible();
  expect(scripts.some((url) => /\/AddRepositoryDialog-/.test(url))).toBe(true);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "添加仓库" })).toBeHidden();
  await revealSidebar(page);
  await page.getByRole("link", { name: "设置", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "设置", exact: true }),
  ).toBeVisible();
  expect(scripts.some((url) => /\/Settings-/.test(url))).toBe(true);
});

for (const theme of ["light", "dark"] as const) {
  test(`${theme}: cancelling a delayed form prevents it from opening after its download`, async ({
    page,
    request,
  }, testInfo) => {
    const prefs = await (await request.get("/api/settings")).json();
    await request.patch("/api/settings", {
      data: { baseRevision: prefs.revision, changes: { theme } },
    });
    const requested = Promise.withResolvers<void>();
    const release = Promise.withResolvers<void>();
    await page.route("**/assets/NewSessionDialog-*.js", async (route) => {
      requested.resolve();
      await release.promise;
      await route.continue();
    });
    await page.goto("/");
    await page
      .getByRole("main")
      .getByRole("button", { name: "＋ 新建会话", exact: true })
      .first()
      .click();
    await requested.promise;
    try {
      await expect(
        page.getByRole("status").filter({ hasText: "正在加载创建表单" }),
      ).toBeVisible();
      await page.screenshot({
        path: testInfo.outputPath(`${theme}-pending-form.png`),
      });
      await page.getByRole("button", { name: "取消", exact: true }).click();
    } finally {
      release.resolve();
    }
    await settled(page);
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await page
      .getByRole("main")
      .getByRole("button", { name: "＋ 新建会话", exact: true })
      .first()
      .click();
    await expect(page.getByRole("dialog")).toBeVisible();
  });

  test(`${theme}: a failed form download leaves the workspace usable`, async ({
    page,
    request,
  }, testInfo) => {
    const prefs = await (await request.get("/api/settings")).json();
    await request.patch("/api/settings", {
      data: { baseRevision: prefs.revision, changes: { theme } },
    });
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.route("**/assets/NewSessionDialog-*.js", (route) =>
      route.abort(),
    );
    await page.goto("/");
    await page
      .getByRole("main")
      .getByRole("button", { name: "＋ 新建会话", exact: true })
      .first()
      .click();
    await expect(page.getByRole("alert")).toContainText("创建表单加载失败");
    await page.screenshot({
      path: testInfo.outputPath(`${theme}-failed-form.png`),
    });
    await page.getByRole("button", { name: "关闭", exact: true }).click();
    await expect(page.getByRole("alert")).toHaveCount(0);
    await revealSidebar(page);
    await page.getByRole("link", { name: "设置", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "设置", exact: true }),
    ).toBeVisible();
    expect(errors).toEqual([]);
  });
}

test("a failed settings download reports an error without an unhandled rejection", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/assets/Settings-*.js", (route) => route.abort());
  await page.goto("/settings");
  await expect(page.getByRole("alert")).toContainText("设置加载失败");
  expect(errors).toEqual([]);
});
