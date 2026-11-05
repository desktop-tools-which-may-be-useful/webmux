import type {
  AgentConnector,
  ChatEligibility,
  ChatState,
  HybridPhase,
  HybridSupport,
  SessionHybrid,
} from "./schemas";

// User-facing text for the server's hybrid support reason codes. The rules
// themselves stay on the server (agent profiles); only the wording lives here.
const hybridSupportReasons: Record<
  NonNullable<HybridSupport["reason"]>,
  string
> = {
  unknown_agent: "未识别到支持的 Agent",
  unsupported_form: "命令写法不受支持",
  session_store: "该参数使聊天视图无法找到会话",
  directory_arg: "请在新建会话时选择目录，不要在命令中写目录",
  non_interactive: "该参数不启动交互界面",
  session_arg: "该参数打开已有会话，混合配置请不要指定会话",
};

export function hybridSupportReason(support: HybridSupport | null) {
  if (!support || support.supported) return "";
  const reason = support.reason ?? "unknown_agent";
  const text = hybridSupportReasons[reason];
  return reason === "unknown_agent" && support.supportedAgents.length > 0
    ? `${text}，当前版本支持：${support.supportedAgents.join("、")}`
    : text;
}

export type HybridSideLabel = "终端" | "聊天" | "切换中";

/** Where a hybrid session currently runs, as shown on cards and pages. */
export function hybridSideLabel(hybrid: SessionHybrid): HybridSideLabel {
  if (hybrid.side === "switching" || hybrid.phase) return "切换中";
  if (hybrid.side === "acp") return "聊天";
  return "终端";
}

/** Why the chat side cannot start: its connector is not installed. */
export function connectorMissingText(connector: AgentConnector) {
  return `未安装 ${connector.title}（${connector.executable}）`;
}

/** The notice that a hybrid config's chat side is installed separately. */
export function connectorNoticeText(connector: AgentConnector) {
  return `聊天端需要另外安装 ${connector.executable}`;
}

/** A component running below the version WebMux was verified with. */
export function outdatedText(item: ChatState["outdated"][number]) {
  return `${item.name} ${item.version} 低于已验证版本 ${item.verified}，部分功能可能不可用`;
}

// Why switching to chat is unavailable, in the order the server checks the
// conditions (agent-session-identity); the server sends only the code.
const eligibilityReasons: Record<string, string> = {
  unknown_agent: "未识别到支持的 Agent",
  not_root: "Agent 不是由本会话直接启动的",
  no_session_id: "请先在 TUI 中发送一条消息",
  not_listed: "会话尚未保存或找不到会话",
  occupied: "该会话已在另一个会话中打开",
  occupied_external: "该会话正被另一个 Agent 客户端使用",
  session_ambiguous: "无法确定当前会话，请选择要打开的会话",
  checking: "正在核对会话",
};

/** The reason switching to chat is unavailable, or "" when it is. */
export function eligibilityText(eligibility: ChatEligibility) {
  if (eligibility.available) return "";
  const reason = eligibility.reason ?? "";
  if (reason === "connector_missing" && eligibility.connector)
    return connectorMissingText(eligibility.connector);
  return eligibilityReasons[reason] ?? (reason || "暂时不能切到聊天");
}

export type SwitchTarget = "acp" | "tui";

/**
 * Where a switch stands on this device: a server phase, "requested" before
 * the server reports one, or "arriving" once the terminal run has started
 * and the page waits for its first output.
 */
export type SwitchProgressPhase = HybridPhase | "requested" | "arriving";

type StepPhase = Exclude<SwitchProgressPhase, "requested">;

const phaseLabels: Record<StepPhase, string> = {
  interrupting: "中断中",
  awaiting_choice: "中断中",
  cancelling: "中断中",
  stopping: "退出中",
  starting: "启动中",
  loading: "加载历史中",
  rolling_back: "切换失败，正在回到原端",
  arriving: "连接终端",
};
const stepOrder: Record<SwitchTarget, StepPhase[]> = {
  acp: ["interrupting", "stopping", "starting", "loading"],
  tui: ["cancelling", "stopping", "starting", "arriving"],
};

/**
 * Which way a running switch goes: its phase names it for the steps only one
 * direction has, otherwise the side it left does. Null when neither tells.
 */
export function switchTarget(
  phase: HybridPhase | undefined,
  from: SwitchTarget | undefined,
): SwitchTarget | null {
  if (phase === "interrupting" || phase === "awaiting_choice") return "acp";
  if (phase === "loading") return "acp";
  if (phase === "cancelling") return "tui";
  if (from === "tui") return "acp";
  if (from === "acp") return "tui";
  return null;
}

export type SwitchStep = {
  label: string;
  state: "done" | "current" | "pending";
};

/**
 * The steps of a switch in progress, with the current phase marked; a
 * request the server has not reported yet stands at the first step. A phase
 * outside the direction's steps (a rollback, or an unknown direction) is
 * shown on its own.
 */
export function switchSteps(
  to: SwitchTarget | null,
  phase: SwitchProgressPhase,
): SwitchStep[] {
  const order = to ? stepOrder[to] : [];
  if (phase === "requested")
    return order.map((step, i) => ({
      label: phaseLabels[step],
      state: i === 0 ? "current" : "pending",
    }));
  const at = order.indexOf(
    phase === "awaiting_choice" ? "interrupting" : phase,
  );
  if (at < 0) return [{ label: phaseLabels[phase], state: "current" }];
  return order.map((step, i) => ({
    label: phaseLabels[step],
    state: i < at ? "done" : i === at ? "current" : "pending",
  }));
}

/** An agent session ID shortened for notices. */
export function shortId(id: string) {
  return id.length > 12 ? `${id.slice(0, 8)}…` : id;
}
