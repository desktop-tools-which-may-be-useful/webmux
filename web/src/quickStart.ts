import { z } from "zod";
import { api, APIError, store, stored } from "./api";
import { newRequestId } from "./ids";
import {
  QuickStartFailureSchema,
  QuickStartRequestSchema,
  QuickStartResultSchema,
  type ChatConfig,
  type QuickStartFailure,
  type QuickStartRequest,
} from "./schemas";

// Quick start's memory on this device: the repository and agent of the last
// successful start, and the unsent text. Both are validated on read and
// degrade to nothing when unreadable.

const ChoiceSchema = z.object({
  repositoryId: z.string().min(1).max(256),
  configId: z.string().regex(/^[a-z2-7]{8}$/),
});
export type QuickStartChoice = z.infer<typeof ChoiceSchema>;
const DraftSchema = z.object({ text: z.string().max(100_000) });

function readJson<T extends z.ZodType>(key: string, schema: T) {
  try {
    return schema.safeParse(JSON.parse(stored(key, "null"))).data;
  } catch {
    return undefined;
  }
}

export const quickStartMemory = {
  choice: (): QuickStartChoice | undefined =>
    readJson("quickStart", ChoiceSchema),
  rememberChoice(choice: QuickStartChoice) {
    store("quickStart", JSON.stringify(choice));
  },
  draft: (): string => readJson("quick-start-draft", DraftSchema)?.text ?? "",
  saveDraft(text: string) {
    if (text) store("quick-start-draft", JSON.stringify({ text }));
    else quickStartMemory.clearDraft();
  },
  clearDraft() {
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
  | { ok: false; failure: QuickStartFailure };

/**
 * Sends one quick start. A dropped connection is retried once with the same
 * request ID, which the server answers with the first run's result.
 */
export async function sendQuickStart(
  request: Omit<QuickStartRequest, "clientRequestId">,
  clientRequestId = newRequestId(),
): Promise<QuickStartOutcome> {
  const body = { ...request, clientRequestId };
  for (let attempt = 0; ; attempt++) {
    try {
      const result = await api(
        "/quick-starts",
        QuickStartResultSchema,
        "POST",
        body,
        undefined,
        QuickStartRequestSchema,
      );
      return { ok: true, sessionId: result.sessionId };
    } catch (err) {
      if (err instanceof TypeError && attempt === 0) continue;
      const failure =
        err instanceof APIError
          ? QuickStartFailureSchema.safeParse(err.body).data
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

export const failedStep: Record<QuickStartFailure["step"], string> = {
  precheck: "无法开始",
  worktree: "创建 worktree 失败",
  session: "启动 agent 失败",
  config: "应用配置失败",
  prompt: "发送首条消息失败",
};
