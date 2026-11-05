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

test("chats and archived records share one list, newest first", async () => {
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
    "整理文档（已结束）",
    "等授权（等待）",
    "空闲的聊天（空闲）",
  ]);
  await expect
    .element(list().getByRole("button", { name: "等授权（等待）" }))
    .toHaveAttribute("aria-current", "page");
  await list().getByRole("button", { name: "工作中的聊天（工作中）" }).click();
  expect(onopen).toHaveBeenCalledWith(working);
  await list().getByRole("button", { name: "整理文档（已结束）" }).click();
  expect(onresume).toHaveBeenCalledWith(
    expect.objectContaining({ id: "archivea" }),
  );
});
