import {
  createShell,
  expect,
  test,
  backToList,
  revealSidebar,
} from "./fixtures";

test("the saved dark theme is applied before the application module loads", async ({
  page,
}) => {
  const moduleGate = Promise.withResolvers<void>();
  let delayed = false;
  await page.addInitScript(() => {
    localStorage.setItem("webmux.theme", "dark");
  });
  await page.route("**/assets/*.js", async (route) => {
    if (!delayed) {
      delayed = true;
      await moduleGate.promise;
    }
    await route.continue();
  });
  try {
    await page.goto("/", { waitUntil: "commit" });
    await expect(page.locator("html")).toHaveClass(/(?:^|\s)dark(?:\s|$)/);
    await expect(
      page
        .getByRole("main")
        .getByRole("button", { name: "＋ 新建会话" })
        .first(),
    ).toHaveCount(0);
  } finally {
    moduleGate.resolve();
  }
});

test("reduced motion disables skeleton and dialog animation", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  // Server settings still gate the startup skeleton; Git discovery does not.
  const config = Promise.withResolvers<void>();
  await page.route("**/api/config", async (route) => {
    await config.promise;
    await route.continue();
  });
  try {
    await page.goto("/");
    const skeleton = page.locator('[data-slot="skeleton"]').first();
    await expect(skeleton).toBeVisible();
    expect(
      await skeleton.evaluate(
        (node) => getComputedStyle(node).animationIterationCount,
      ),
    ).toBe("1");
    expect(
      await skeleton.evaluate((node) =>
        parseFloat(getComputedStyle(node).animationDuration),
      ),
    ).toBeLessThanOrEqual(0.001);
    // The page body is still a skeleton; the sidebar already offers it.
    await revealSidebar(page);
    await page.getByRole("button", { name: "新建会话", exact: true }).click();
    await expect(page.getByRole("dialog", { name: "导航" })).toHaveCount(0);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    expect(
      await dialog.evaluate((node) =>
        parseFloat(getComputedStyle(node).transitionDuration),
      ),
    ).toBeLessThanOrEqual(0.001);
  } finally {
    config.resolve();
  }
});

test("returning to the workspace focuses its main region", async ({
  page,
  workDir,
}) => {
  await createShell(page, "Route focus", workDir);
  await backToList(page);
  await expect(page.getByRole("searchbox", { name: "搜索会话" })).toBeVisible();
  await expect(page.locator("main")).toBeFocused();
});

test("returning to the workspace shows the previous list while settings reload", async ({
  page,
  workDir,
}) => {
  await createShell(page, "Cached return", workDir);
  await backToList(page);
  await expect(page.getByRole("searchbox", { name: "搜索会话" })).toBeVisible();
  // Open the session again, then hold the slow settings and Git reads that
  // a mobile link would still be waiting on.
  await page.getByRole("main").getByText("Cached return").first().click();
  const config = Promise.withResolvers<void>();
  const repositories = Promise.withResolvers<void>();
  await page.route("**/api/config", async (route) => {
    await config.promise;
    await route.continue();
  });
  await page.route("**/api/repositories", async (route) => {
    await repositories.promise;
    await route.continue();
  });
  try {
    await backToList(page);
    await expect(
      page.getByRole("searchbox", { name: "搜索会话" }),
    ).toBeVisible();
    await expect(
      page.getByRole("main").getByText("Cached return").first(),
    ).toBeVisible();
    await expect(page.locator('[data-slot="skeleton"]')).toHaveCount(0);
  } finally {
    config.resolve();
    repositories.resolve();
  }
});
