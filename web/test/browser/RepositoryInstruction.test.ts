import { expect, test, vi } from "vitest";
import { page } from "vitest/browser";
import { render } from "vitest-browser-svelte/pure";
import AddRepositoryDialog from "../../src/components/AddRepositoryDialog.svelte";
import InitInstructionDialog from "../../src/components/InitInstructionDialog.svelte";
import {
  RegisterRepositoryRequestSchema,
  UpdateRepositoryRequestSchema,
} from "../../src/schemas";
import { commands, installChatFixture, respond } from "./chat-fixture";
import { repository } from "./shell-fixture";

installChatFixture();

test("adding a repository saves its init instruction", async () => {
  respond(/^POST \/api\/repositories$/, ({ body }) =>
    Response.json(
      repository({
        initInstruction:
          RegisterRepositoryRequestSchema.parse(body).initInstruction ?? "",
      }),
      { status: 201 },
    ),
  );
  const onclose = vi.fn();
  render(AddRepositoryDialog, { defaultCwd: "~", onclose, onchange: vi.fn() });
  await page.getByLabelText("仓库目录").fill("/repos/webmux");
  await page
    .getByLabelText("初始化指令（可选）")
    .fill("  先读 AGENTS.md，改完运行 make check  ");
  await page.getByRole("button", { name: "添加仓库" }).click();
  await vi.waitFor(() => expect(onclose).toHaveBeenCalled());
  expect(commands().at(-1)?.body).toEqual({
    path: "/repos/webmux",
    initInstruction: "先读 AGENTS.md，改完运行 make check",
  });
});

test("registering again keeps the old instruction and says where to edit it", async () => {
  respond(/^POST \/api\/repositories$/, () =>
    Response.json(repository({ initInstruction: "旧指令" })),
  );
  const onclose = vi.fn();
  const onchange = vi.fn();
  render(AddRepositoryDialog, { defaultCwd: "~", onclose, onchange });
  await page.getByLabelText("仓库目录").fill("/repos/webmux");
  await page.getByLabelText("初始化指令（可选）").fill("新指令");
  await page.getByRole("button", { name: "添加仓库" }).click();
  await expect
    .element(page.getByRole("status"))
    .toMatchTextContent(/原有的初始化指令没有改变.*初始化指令…/);
  expect(onchange).toHaveBeenCalled();
  expect(onclose).not.toHaveBeenCalled();
});

test("the limit blocks an over-long instruction before sending", async () => {
  render(AddRepositoryDialog, {
    defaultCwd: "~",
    onclose: vi.fn(),
    onchange: vi.fn(),
  });
  await page.getByLabelText("仓库目录").fill("/repos/webmux");
  await page.getByLabelText("初始化指令（可选）").fill("长".repeat(4001));
  await expect
    .element(page.getByRole("button", { name: "添加仓库" }))
    .toBeDisabled();
  await expect.element(page.getByText("4001 / 4000")).toBeVisible();
});

test("the menu dialog edits or clears the instruction", async () => {
  respond(/^PATCH \/api\/repositories\/repo1$/, ({ body }) =>
    Response.json(
      repository({
        initInstruction:
          UpdateRepositoryRequestSchema.parse(body).initInstruction,
      }),
    ),
  );
  const onclose = vi.fn();
  render(InitInstructionDialog, {
    repository: repository({ initInstruction: "先读 AGENTS.md" }),
    onclose,
    onchange: vi.fn(),
  });
  const field = page.getByRole("textbox", { name: "初始化指令" });
  await expect.element(field).toHaveValue("先读 AGENTS.md");
  await field.fill("");
  await page.getByRole("button", { name: "保存" }).click();
  await vi.waitFor(() => expect(onclose).toHaveBeenCalled());
  expect(commands().at(-1)).toMatchObject({
    method: "PATCH",
    path: "/api/repositories/repo1",
    body: { initInstruction: "" },
  });
});

test("a rejected save keeps the dialog open with the reason", async () => {
  respond(/^PATCH \/api\/repositories\/repo1$/, () =>
    Response.json({ error: "初始化指令最多 4000 个字符" }, { status: 400 }),
  );
  const onclose = vi.fn();
  render(InitInstructionDialog, {
    repository: repository(),
    onclose,
    onchange: vi.fn(),
  });
  await page.getByRole("textbox", { name: "初始化指令" }).fill("新指令");
  await page.getByRole("button", { name: "保存" }).click();
  await expect
    .element(page.getByRole("alert"))
    .toMatchTextContent(/最多 4000 个字符/);
  expect(onclose).not.toHaveBeenCalled();
});
