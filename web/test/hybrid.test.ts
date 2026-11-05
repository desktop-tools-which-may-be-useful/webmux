import { describe, expect, test } from "vitest";
import {
  connectorMissingText,
  connectorNoticeText,
  eligibilityText,
  hybridSideLabel,
  hybridSupportReason,
  outdatedText,
  shortId,
  switchSteps,
  switchTarget,
} from "../src/hybrid";

const connector = {
  executable: "codex-acp",
  title: "Codex ACP 适配器",
  installCommand: "npm install -g @agentclientprotocol/codex-acp",
  docsUrl: "https://github.com/agentclientprotocol/codex-acp",
};

const hybrid = {
  side: "acp" as const,
  state: "active" as const,
  agent: "omp",
  agentTitle: "omp",
  agentIcon: "pi",
  agentSessionId: null,
};

describe("hybrid labels", () => {
  test("a phase means switching even before the side flips", () => {
    expect(hybridSideLabel({ ...hybrid, side: "tui" })).toBe("终端");
    expect(hybridSideLabel(hybrid)).toBe("聊天");
    expect(hybridSideLabel({ ...hybrid, side: "switching" })).toBe("切换中");
    expect(hybridSideLabel({ ...hybrid, side: "tui", phase: "stopping" })).toBe(
      "切换中",
    );
  });

  test("support and eligibility reasons fall back for unknown codes", () => {
    expect(hybridSupportReason(null)).toBe("");
    expect(
      hybridSupportReason({
        supported: true,
        supportedAgents: [],
        agent: "omp",
      }),
    ).toBe("");
    expect(hybridSupportReason({ supported: false, supportedAgents: [] })).toBe(
      "未识别到支持的 Agent",
    );
    expect(
      hybridSupportReason({
        supported: false,
        supportedAgents: ["A", "B"],
        reason: "unknown_agent",
      }),
    ).toBe("未识别到支持的 Agent，当前版本支持：A、B");
    expect(eligibilityText({ available: true, reason: "occupied" })).toBe("");
    expect(eligibilityText({ available: false, reason: "occupied" })).toBe(
      "该会话已在另一个会话中打开",
    );
    expect(eligibilityText({ available: false, reason: "future" })).toBe(
      "future",
    );
    expect(eligibilityText({ available: false })).toBe("暂时不能切到聊天");
  });

  test("Codex reasons name the connector and the other client", () => {
    expect(
      hybridSupportReason({
        supported: false,
        supportedAgents: [],
        reason: "session_arg",
      }),
    ).toBe("该参数打开已有会话，混合配置请不要指定会话");
    expect(connectorMissingText(connector)).toBe(
      "未安装 Codex ACP 适配器（codex-acp）",
    );
    expect(connectorNoticeText(connector)).toBe("聊天端需要另外安装 codex-acp");
    expect(
      eligibilityText({
        available: false,
        reason: "connector_missing",
        connector,
      }),
    ).toBe("未安装 Codex ACP 适配器（codex-acp）");
    expect(
      eligibilityText({ available: false, reason: "occupied_external" }),
    ).toBe("该会话正被另一个 Agent 客户端使用");
    expect(
      outdatedText({ name: "codex-acp", version: "2.4.0", verified: "2.5.0" }),
    ).toBe("codex-acp 2.4.0 低于已验证版本 2.5.0，部分功能可能不可用");
  });

  test("short IDs keep short ones whole", () => {
    expect(shortId("ses_e7fb6e69effeUOvdil8kU5qkcf")).toBe("ses_e7fb…");
    expect(shortId("01a")).toBe("01a");
  });
});

describe("switch progress", () => {
  test("the direction comes from one-way phases, else the side left", () => {
    expect(switchTarget("awaiting_choice", undefined)).toBe("acp");
    expect(switchTarget("cancelling", undefined)).toBe("tui");
    expect(switchTarget("stopping", "acp")).toBe("tui");
    expect(switchTarget("starting", "tui")).toBe("acp");
    expect(switchTarget("stopping", undefined)).toBeNull();
  });

  test("steps mark done, current and pending; awaiting a choice is interrupting", () => {
    expect(switchSteps("acp", "stopping")).toEqual([
      { label: "中断中", state: "done" },
      { label: "退出中", state: "current" },
      { label: "启动中", state: "pending" },
      { label: "加载历史中", state: "pending" },
    ]);
    expect(switchSteps("acp", "awaiting_choice")[0]).toEqual({
      label: "中断中",
      state: "current",
    });
    expect(switchSteps("tui", "starting").map((s) => s.state)).toEqual([
      "done",
      "done",
      "current",
      "pending",
    ]);
  });

  test("a switch to the terminal ends by connecting it; a request starts at the first step", () => {
    expect(switchSteps("tui", "arriving")).toEqual([
      { label: "中断中", state: "done" },
      { label: "退出中", state: "done" },
      { label: "启动中", state: "done" },
      { label: "连接终端", state: "current" },
    ]);
    expect(switchSteps("acp", "requested").map((s) => s.state)).toEqual([
      "current",
      "pending",
      "pending",
      "pending",
    ]);
    expect(switchSteps(null, "requested")).toEqual([]);
  });

  test("a rollback or unknown direction is shown on its own", () => {
    expect(switchSteps("acp", "rolling_back")).toEqual([
      { label: "切换失败，正在回到原端", state: "current" },
    ]);
    expect(switchSteps(null, "stopping")).toEqual([
      { label: "退出中", state: "current" },
    ]);
  });
});
