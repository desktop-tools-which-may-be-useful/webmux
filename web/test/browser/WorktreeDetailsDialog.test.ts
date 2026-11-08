import { expect, test, vi } from "vitest";
import { page } from "vitest/browser";
import { render } from "vitest-browser-svelte/pure";
import WorktreeDetailsDialog from "../../src/components/WorktreeDetailsDialog.svelte";
import { WorktreeUsageSchema } from "../../src/schemas";
import { commands, installChatFixture, respond } from "./chat-fixture";
import { worktree } from "./shell-fixture";

installChatFixture();

const usagePath = /^POST \/api\/repositories\/[^/]+\/worktrees\/[^/]+\/usage$/;

function open(patch: Parameters<typeof worktree>[0] = {}) {
  const tree = worktree(patch);
  render(WorktreeDetailsDialog, {
    title: "main",
    path: tree.path,
    worktree: tree,
    sessions: [],
    onclose: vi.fn(),
  });
  return tree;
}

test("the size is only walked when asked for, and a partial count says so", async () => {
  respond(usagePath, () =>
    Response.json(
      WorktreeUsageSchema.parse({
        bytes: 5 * 1024 * 1024,
        entries: 12345,
        complete: false,
      }),
    ),
  );
  open();
  const measure = page.getByRole("button", { name: "计算容量" });
  await expect.element(measure).toBeVisible();
  expect(commands()).toHaveLength(0);
  await measure.click();
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("5 MiB · 12,345 个条目（未统计完整，实际更大）");
  expect(commands()).toHaveLength(1);
  expect(commands()[0]).toMatchObject({
    method: "POST",
    path: "/api/repositories/repo1/worktrees/main/usage",
  });
  await expect
    .element(page.getByRole("button", { name: "重新计算" }))
    .toBeEnabled();
});

test("a failed measurement shows the error and can be retried", async () => {
  respond(usagePath, () =>
    Response.json({ error: "无法读取工作树目录" }, { status: 503 }),
  );
  open();
  await page.getByRole("button", { name: "计算容量" }).click();
  await expect
    .element(page.getByRole("alert"))
    .toHaveTextContent("无法读取工作树目录");
  await expect
    .element(page.getByRole("button", { name: "计算容量" }))
    .toBeEnabled();
});

test("a worktree whose directory is missing cannot be measured", async () => {
  open({ available: false });
  await expect
    .element(page.getByRole("button", { name: "计算容量" }))
    .toBeDisabled();
});
