<script lang="ts">
  import WorkspaceGroup from "../../src/components/WorkspaceGroup.svelte";
  import type { ArchiveActions } from "../../src/archived";
  import type { HybridRecord, SessionInfo } from "../../src/schemas";
  import { now, session } from "../fixtures";
  import { agentMetadata, sessionIds } from "./fixtures";

  // The home page's workspace group with hybrid sessions on each side beside
  // a plain shell: the card names the side and, on the chat side, previews
  // the latest agent message instead of terminal lines. A running turn
  // shows its live one-line work detail above the preview. With `archived`,
  // the group also lists archived records, the first one just archived.
  let { archived = false }: { archived?: boolean } = $props();
  let notice = $state("");
  const record = (
    id: string,
    title: string,
    patch: Partial<HybridRecord> = {},
  ): HybridRecord => ({
    id,
    name: title,
    title,
    configId: "cfgomp01",
    configMissing: false,
    agent: "omp",
    cwd: "/home/developer/webmux",
    cmd: ["omp"],
    side: "acp",
    agentSessionId: sessionIds.omp.current,
    previousIds: [],
    state: "closed",
    queuedCount: 0,
    createdAt: "2026-01-01T09:00:00Z",
    updatedAt: "2026-01-01T11:58:00Z",
    interruptedAt: null,
    ...patch,
  });
  const records: HybridRecord[] = [
    record(
      "archivd1",
      "omp · 修复首页排序后补充回归测试与文档说明，确认长标题会截断",
    ),
    record("archivd2", "OpenCode · 升级依赖", {
      agent: "opencode",
      cmd: ["opencode"],
      side: "tui",
      state: "interrupted",
      activity: "turn",
      queuedCount: 2,
      interruptedAt: "2026-01-01T10:00:00Z",
      updatedAt: "2026-01-01T10:00:00Z",
    }),
    record("archivd3", "omp · web 子目录的样式调整", {
      cwd: "/home/developer/webmux/web",
      updatedAt: "2025-12-30T12:00:00Z",
    }),
  ];
  const archive: ArchiveActions = {
    now,
    filtering: false,
    resume: async () => {
      notice = "此预览使用示例数据，未恢复真实会话。";
      return new Promise(() => {});
    },
    remove: async () => {
      notice = "此预览使用示例数据，未删除真实记录。";
    },
    onresumed: () => {},
  };
  const base = {
    ...session,
    cwd: "/home/developer/webmux",
    reportedCwd: null,
    clients: 0,
  };
  const hybrid = (
    agent: "omp" | "opencode",
    side: "acp" | "tui" | "switching",
    patch: Partial<NonNullable<SessionInfo["hybrid"]>> = {},
  ) => ({
    side,
    dormant: false,
    state: "active" as const,
    agent,
    agentTitle: agentMetadata[agent].title,
    agentIcon: agentMetadata[agent].icon,
    agentSessionId: sessionIds[agent].current,
    ...patch,
  });
  const sessions: SessionInfo[] = [
    {
      ...base,
      id: "hybridac",
      generation: "",
      title: "omp · 修复首页排序",
      cmd: ["omp"],
      status: "waiting",
      attention: {
        kind: "permission",
        source: "agent",
        since: "2026-01-01T11:59:00Z",
      },
      preview:
        "排序只比较了最近活动时间，没有考虑置顶。我先补测试，再修改 sortWorkspaces。需要授权运行 pnpm --dir web exec vitest。",
      hybrid: hybrid("omp", "acp", { work: "pnpm --dir web exec vitest" }),
    },
    {
      ...base,
      id: "hybridwk",
      generation: "",
      title: "OpenCode · 排查工具调用",
      cmd: ["opencode"],
      status: "running",
      attention: null,
      preview: "",
      hybrid: hybrid("opencode", "acp", {
        work: "读取 internal/hybrid/work.go",
      }),
    },
    {
      ...base,
      id: "hybridtu",
      title: "OpenCode · 更新 README",
      cmd: ["opencode"],
      status: "running",
      attention: null,
      preview: [
        "› 继续：更新 README 中的排序说明",
        "",
        "  ✓ Read README.md",
        "  ● Editing README.md …",
      ].join("\n"),
      hybrid: hybrid("opencode", "tui"),
    },
    {
      ...base,
      id: "hybridsl",
      generation: "",
      title: "omp · 整理测试",
      cmd: ["omp"],
      status: "idle",
      attention: null,
      preview: "测试已整理完毕，全部通过。",
      hybrid: hybrid("omp", "acp", { dormant: true }),
    },
    {
      ...base,
      id: "plainshl",
      title: "bash",
      cmd: ["/bin/bash"],
      status: "idle",
      attention: null,
      preview: "$ git status\nOn branch main\nnothing to commit",
    },
  ];
</script>

<main class="mx-auto max-w-5xl px-4 py-8 sm:px-8 sm:py-12">
  <WorkspaceGroup
    title="webmux"
    path="/home/developer/webmux"
    {sessions}
    {now}
    onclose={() => (notice = "此预览使用示例数据，未关闭真实会话。")}
    archived={archived ? records : []}
    {archive}
  />
  {#if notice}<p role="status" class="mt-4 text-sm text-muted-foreground">
      {notice}
    </p>{/if}
</main>
