import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import ChatHistory from "../../src/components/ChatHistory.svelte";
import { chat, launchConfig, record, session } from "./shell-fixture";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
});

const list = () => page.getByRole("list", { name: "聊天历史" });
const archivedList = () => page.getByRole("list", { name: "已归档聊天" });

test("live chats show newest first and archived records fold below", async () => {
  const onopen = vi.fn();
  const onresume = vi.fn();
  const working = chat({
    id: "chataaaa",
    title: "工作中的聊天",
    status: "running",
    lastActiveAt: "2026-10-28T03:00:00Z",
  });
  render(ChatHistory, {
    sessions: [
      working,
      chat({
        id: "chatbbbb",
        title: "等授权",
        status: "waiting",
        lastActiveAt: "2026-10-28T02:00:00Z",
        attention: {
          kind: "permission",
          source: "agent",
          since: "2026-10-28T02:00:00Z",
        },
      }),
      chat({
        id: "chatcccc",
        title: "空闲的聊天",
        lastActiveAt: "2026-10-28T01:00:00Z",
      }),
      // Terminal sessions never show here.
      session({ id: "shellaaa", title: "pnpm dev" }),
    ],
    records: [
      record({ id: "chatcccc", title: "旧记录" }),
      record({
        id: "archivea",
        title: "整理文档",
        updatedAt: "2026-10-28T02:30:00Z",
      }),
    ],
    configs: [launchConfig()],
    current: "chatbbbb",
    onopen,
    onresume,
    onrename: vi.fn(),
    onclose: vi.fn(),
    onremove: vi.fn(),
  });
  const rows = list().getByRole("button", { name: /（/ });
  await expect.element(rows.first()).toBeVisible();
  expect(rows.elements().map((row) => row.getAttribute("aria-label"))).toEqual([
    "工作中的聊天（工作中）",
    "等授权（等待）",
    "空闲的聊天（空闲）",
  ]);
  await expect
    .element(list().getByRole("button", { name: "等授权（等待）" }))
    .toHaveAttribute("aria-current", "page");
  await list().getByRole("button", { name: "工作中的聊天（工作中）" }).click();
  expect(onopen).toHaveBeenCalledWith(working);

  // Archived records are collapsed by default; expanding shows them.
  const trigger = page.getByRole("button", { name: /已归档/ });
  await expect.element(trigger).toHaveAttribute("aria-expanded", "false");
  await trigger.click();
  await expect.element(trigger).toHaveAttribute("aria-expanded", "true");
  await archivedList()
    .getByRole("button", { name: "整理文档（已结束）" })
    .click();
  expect(onresume).toHaveBeenCalledWith(
    expect.objectContaining({ id: "archivea" }),
  );
});

test("archived section opens automatically when viewing or resuming an archived record", async () => {
  render(ChatHistory, {
    sessions: [],
    records: [
      record({
        id: "archivea",
        title: "整理文档",
        updatedAt: "2026-10-28T02:30:00Z",
      }),
    ],
    configs: [launchConfig()],
    current: "archivea",
    onopen: vi.fn(),
    onresume: vi.fn(),
    onrename: vi.fn(),
    onclose: vi.fn(),
    onremove: vi.fn(),
  });
  const trigger = page.getByRole("button", { name: /已归档/ });
  await expect.element(trigger).toHaveAttribute("aria-expanded", "true");
  await expect
    .element(archivedList().getByRole("button", { name: "整理文档（已结束）" }))
    .toHaveAttribute("aria-current", "page");
});
