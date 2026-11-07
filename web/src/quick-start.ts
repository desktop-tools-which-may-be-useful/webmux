import { z } from "zod";
import { api, APIError, store, stored } from "./api";
import type { ConfigValue } from "./chat-api";
import { newRequestId } from "./ids";
import {
  ChatAttachmentSchema,
  ChatPermissionModeSchema,
  CreateSessionFailureSchema,
  CreateSessionRequestSchema,
  CreateSessionResultSchema,
  type ChatAttachment,
  type ChatConfig,
  type ChatPermissionMode,
  type CreateSessionFailure,
  type CreateSessionRequest,
} from "./schemas";

// Quick start's memory on this device: the repository and agent of the last
// successful start, and the unsent composer state. Both are validated on read
// and degrade to nothing when unreadable.

const ChoiceSchema = z.object({
  repositoryId: z.string().min(1).max(256),
  configId: z.string().regex(/^[a-z2-7]{8}$/),
});
export type QuickStartChoice = z.infer<typeof ChoiceSchema>;

const ConfigValueSchema: z.ZodType<ConfigValue> = z.union([
  z.object({ value: z.string().max(1024) }),
  z.object({ enabled: z.boolean() }),
]);

const BaseChoiceSchema = z.object({
  repositoryId: z.string().min(1).max(256),
  name: z.string().min(1).max(256),
});

const DraftSchema = z.object({
  text: z.string().max(100_000).default(""),
  attachments: z.array(ChatAttachmentSchema).max(8).optional(),
  repositoryId: z.string().min(1).max(256).optional(),
  configId: z
    .string()
    .regex(/^[a-z2-7]{8}$/)
    .optional(),
  baseChoice: BaseChoiceSchema.optional(),
  permissionMode: ChatPermissionModeSchema.optional(),
  edits: z
    .record(z.string(), z.record(z.string(), ConfigValueSchema))
    .optional(),
  unticked: z.string().min(1).max(256).optional(),
});
export type QuickStartDraft = {
  text: string;
  attachments?: ChatAttachment[];
  repositoryId?: string;
  configId?: string;
  baseChoice?: { repositoryId: string; name: string };
  permissionMode?: ChatPermissionMode;
  edits?: Record<string, Record<string, ConfigValue>>;
  unticked?: string;
};

let memoryDraft: QuickStartDraft | undefined;

function readJson<T extends z.ZodType>(key: string, schema: T) {
  try {
    return schema.safeParse(JSON.parse(stored(key, "null"))).data;
  } catch {
    return undefined;
  }
}

function emptyDraft(draft: QuickStartDraft) {
  return (
    !draft.text &&
    !draft.attachments?.length &&
    !draft.repositoryId &&
    !draft.configId &&
    !draft.baseChoice &&
    (!draft.permissionMode || draft.permissionMode === "auto") &&
    (!draft.edits || Object.keys(draft.edits).length === 0) &&
    !draft.unticked
  );
}

export const quickStartMemory = {
  choice: (): QuickStartChoice | undefined =>
    readJson("quickStart", ChoiceSchema),
  rememberChoice(choice: QuickStartChoice) {
    store("quickStart", JSON.stringify(choice));
  },
  loadDraft(): QuickStartDraft {
    const raw = stored("quick-start-draft", "");
    if (!raw) {
      // If storage was cleared externally (e.g. between tests), drop the
      // fallback only when localStorage is reachable and empty.
      try {
        if (localStorage.getItem("webmux.quick-start-draft") === null)
          memoryDraft = undefined;
      } catch {
        /* use memoryDraft when storage is unreachable */
      }
      return memoryDraft ?? { text: "" };
    }
    try {
      const parsed = DraftSchema.safeParse(JSON.parse(raw));
      return parsed.success ? parsed.data : { text: "" };
    } catch {
      return { text: "" };
    }
  },
  draft: (): string => quickStartMemory.loadDraft().text,
  saveDraftState(draft: QuickStartDraft) {
    if (emptyDraft(draft)) {
      quickStartMemory.clearDraft();
      return;
    }
    memoryDraft = draft;
    store("quick-start-draft", JSON.stringify(draft));
  },
  saveDraft(text: string) {
    quickStartMemory.saveDraftState({
      ...quickStartMemory.loadDraft(),
      text,
    });
  },
  clearDraft() {
    memoryDraft = undefined;
    try {
      localStorage.removeItem("webmux.quick-start-draft");
    } catch {
      /* private-mode storage is optional */
    }
  },
};

/**
 * The remembered item when it is still offered and usable, else the first
 * usable one; undefined when none is.
 */
export function pickDefault<T extends { id: string }>(
  items: readonly T[],
  remembered: string | undefined,
  usable: (item: T) => boolean,
): T | undefined {
  const kept = items.find((item) => item.id === remembered);
  return kept && usable(kept) ? kept : items.find(usable);
}

/** The values the page shows for each option, as the request sends them. */
export function configValues(options: readonly ChatConfig[]) {
  return Object.fromEntries(
    options.flatMap((option) =>
      option.type === "boolean"
        ? option.enabled === undefined
          ? []
          : [[option.id, String(option.enabled)]]
        : option.value === undefined
          ? []
          : [[option.id, option.value]],
    ),
  );
}

export type QuickStartOutcome =
  | { ok: true; sessionId: string }
  | { ok: false; failure: CreateSessionFailure };

/**
 * Sends one quick start: a session created in a new worktree with its first
 * message. A dropped connection is retried once with the same request ID,
 * which the server answers with the first run's result.
 */
export async function sendQuickStart(
  request: Omit<CreateSessionRequest, "clientRequestId">,
  clientRequestId = newRequestId(),
): Promise<QuickStartOutcome> {
  const body = { ...request, clientRequestId };
  for (let attempt = 0; ; attempt++) {
    try {
      const result = await api(
        "/sessions",
        CreateSessionResultSchema,
        "POST",
        body,
        undefined,
        CreateSessionRequestSchema,
      );
      return { ok: true, sessionId: result.session.id };
    } catch (err) {
      if (err instanceof TypeError && attempt === 0) continue;
      const failure =
        err instanceof APIError
          ? CreateSessionFailureSchema.safeParse(err.body).data
          : undefined;
      return {
        ok: false,
        failure: failure ?? {
          error: err instanceof Error ? err.message : String(err),
          step: "precheck",
        },
      };
    }
  }
}

export const failedStep: Record<CreateSessionFailure["step"], string> = {
  precheck: "无法开始",
  worktree: "创建 worktree 失败",
  session: "启动 agent 失败",
  config: "应用配置失败",
  prompt: "发送首条消息失败",
};
