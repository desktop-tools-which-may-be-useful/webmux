import { QuickCommandsSchema, type QuickCommand } from "./schemas";

export const QUICK_COMMANDS_LIMIT = 50;
export const QUICK_COMMAND_NAME_MAX = 40;

/** Placeholder replaced by the session's artifacts folder path. */
export const DIR_REFERENCE_NAME = "dir";
const DIR_PLACEHOLDER = `{{${DIR_REFERENCE_NAME}}}`;

const ARTIFACT_PRESET_COMMAND = [
  "本会话的临时文件夹：{{dir}}",
  "- 我上传给你的文件在 {{dir}}/uploads/ 中。",
  "- 你生成给我的截图、报告、图表、导出数据等交付物，请保存到 {{dir}}（可以建子目录），不要放进项目仓库。",
  "- 我会在 WebMux 代码栏的“临时文件夹”中查看这些文件。",
  "- 完成后告诉我文件相对于该文件夹的路径。",
  "- 该文件夹会在会话删除后清理，不要存放需要长期保留的内容。",
].join("\n");

/** System-provided quick commands; the defaults while no list is saved. */
export const QUICK_COMMAND_PRESETS: readonly QuickCommand[] = [
  { id: "preset-resume", name: "恢复会话", command: "/resume", enter: true },
  { id: "preset-new", name: "新会话", command: "/new", enter: true },
  {
    id: "preset-artifacts",
    name: "临时文件夹",
    command: ARTIFACT_PRESET_COMMAND,
    enter: false,
  },
];

/** True when running the body needs the session's artifacts folder. */
export function usesArtifactDir(text: string) {
  return text.includes(DIR_PLACEHOLDER);
}

/** Replaces every `{{dir}}` with the folder path verbatim; bodies are prose, not shell. */
export function renderArtifactDir(text: string, dir: string) {
  return text.replaceAll(DIR_PLACEHOLDER, dir);
}

/** Built-in reference `{{user}}`: where the first message goes in an init instruction. */
export const USER_REFERENCE_NAME = "user";
/** Reserved for a future system template; neither usable nor a valid name. */
export const RESERVED_REFERENCE_NAME = "webmux";

/**
 * The first problem with a quick command name among `others`, or "" when it
 * can be saved. Parsing stored lists stays lenient for legacy empty names.
 */
export function quickCommandNameError(name: string, others: readonly string[]) {
  const trimmed = name.trim();
  if (!trimmed) return "请填写名称";
  if (trimmed.length > QUICK_COMMAND_NAME_MAX)
    return `名称不能超过 ${QUICK_COMMAND_NAME_MAX} 个字符`;
  if (/[{}\r\n]/.test(trimmed)) return "名称不能包含 { }或换行";
  if (
    trimmed === USER_REFERENCE_NAME ||
    trimmed === DIR_REFERENCE_NAME ||
    trimmed === RESERVED_REFERENCE_NAME
  )
    return `“${trimmed}”是初始化指令的保留变量，请换一个名称`;
  if (others.some((other) => other.trim() === trimmed))
    return "已有同名快捷指令";
  return "";
}

/** The first name problem across a whole list about to be saved, or "". */
export function quickCommandsNameError(list: readonly QuickCommand[]) {
  for (const [index, cmd] of list.entries()) {
    const problem = quickCommandNameError(
      cmd.name,
      list.filter((_, i) => i !== index).map((other) => other.name),
    );
    if (problem) return `快捷指令“${commandLabel(cmd)}”：${problem}`;
  }
  return "";
}

const REFERENCE = /\{\{([^{}\n]+)\}\}/g;

/** Names referenced as `{{name}}` in an init instruction, in order, unique. */
export function initInstructionReferences(template: string) {
  const names = new Set<string>();
  for (const match of template.matchAll(REFERENCE))
    names.add((match[1] ?? "").trim());
  return [...names];
}

/**
 * Single-pass expansion: each `{{name}}` becomes the matching command's text;
 * unmatched references stay literal and are returned in `missing`. The
 * built-in `{{user}}` stays in the text (the server puts the first message
 * there) and sets `hasUser`; the reserved `{{webmux}}` is returned in
 * `reserved` and stays literal.
 */
export function expandInitInstruction(
  template: string,
  commands: readonly QuickCommand[],
) {
  const missing = new Set<string>();
  let hasUser = false;
  let reserved = false;
  const text = template.replace(REFERENCE, (whole, raw: string) => {
    const name = raw.trim();
    if (name === USER_REFERENCE_NAME) {
      hasUser = true;
      return "{{user}}";
    }
    if (name === DIR_REFERENCE_NAME) return DIR_PLACEHOLDER;
    if (name === RESERVED_REFERENCE_NAME) {
      reserved = true;
      return whole;
    }
    const found = commands.find((cmd) => cmd.name.trim() === name);
    if (found) return found.command;
    missing.add(name);
    return whole;
  });
  return { text, missing: [...missing], hasUser, reserved };
}

/**
 * Parses the stored list. An empty string means the browser never saved a
 * list, so the presets apply; unreadable data degrades to an empty list.
 */
export function parseQuickCommands(raw: string): QuickCommand[] {
  if (!raw) return QUICK_COMMAND_PRESETS.map((cmd) => ({ ...cmd }));
  try {
    const result = QuickCommandsSchema.safeParse(JSON.parse(raw));
    return result.success ? result.data : [];
  } catch {
    return [];
  }
}

/** Unifies line endings and drops trailing newlines; submission is `enter`'s job. */
export function normalizeCommand(text: string) {
  return text.replace(/\r\n?/g, "\n").replace(/\n+$/, "");
}

/** First non-blank line of the body, with a trailing ellipsis for multi-line bodies. */
export function commandPreview(cmd: QuickCommand) {
  const body = cmd.command
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
  return body.length > 1 ? `${body[0]} …` : (body[0] ?? "");
}

export function commandLabel(cmd: QuickCommand) {
  return (
    cmd.name ||
    (cmd.command
      .split("\n")
      .map((line) => line.trim())
      .find(Boolean) ??
      "")
  );
}

export function newQuickCommandId() {
  const bytes = crypto.getRandomValues(new Uint8Array(8));
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join(
    "",
  );
}
