import { describe, expect, test } from "vitest";
import {
  QUICK_COMMAND_PRESETS,
  commandLabel,
  commandPreview,
  expandInitInstruction,
  initInstructionReferences,
  normalizeCommand,
  parseQuickCommands,
  quickCommandNameError,
  quickCommandsNameError,
  renderArtifactDir,
  usesArtifactDir,
} from "../src/quickCommands";
import { QuickCommandsSchema, type QuickCommand } from "../src/schemas";

const command = (overrides: Partial<QuickCommand> = {}): QuickCommand => ({
  id: "a1",
  name: "",
  command: "git status",
  enter: false,
  ...overrides,
});

describe("normalizeCommand", () => {
  test("drops every trailing newline so only `enter` submits", () => {
    expect(normalizeCommand("git status\n\n")).toBe("git status");
    expect(normalizeCommand("git status\r\n\r\n")).toBe("git status");
  });

  test("unifies CRLF and CR without touching leading or inner whitespace", () => {
    expect(normalizeCommand("  first\r\n\tsecond\rthird  ")).toBe(
      "  first\n\tsecond\nthird  ",
    );
  });
});

describe("QuickCommandsSchema", () => {
  test("rejects blank bodies and over-long fields", () => {
    expect(
      QuickCommandsSchema.safeParse([command({ command: " \n " })]).success,
    ).toBe(false);
    expect(
      QuickCommandsSchema.safeParse([command({ command: "x".repeat(4097) })])
        .success,
    ).toBe(false);
    expect(
      QuickCommandsSchema.safeParse([command({ name: "n".repeat(41) })])
        .success,
    ).toBe(false);
  });

  test("caps the list at 50 commands", () => {
    const list = (count: number) =>
      Array.from({ length: count }, (_, index) => command({ id: `c${index}` }));
    expect(QuickCommandsSchema.safeParse(list(50)).success).toBe(true);
    expect(QuickCommandsSchema.safeParse(list(51)).success).toBe(false);
  });
});

describe("parseQuickCommands", () => {
  test("offers every system preset when nothing was ever saved", () => {
    expect(parseQuickCommands("")).toEqual([
      {
        id: "preset-resume",
        name: "恢复会话",
        command: "/resume",
        enter: true,
      },
      { id: "preset-new", name: "新会话", command: "/new", enter: true },
      expect.objectContaining({
        id: "preset-artifacts",
        name: "临时文件夹",
        enter: false,
      }),
    ]);
  });

  test("a saved empty list stays empty instead of restoring presets", () => {
    expect(parseQuickCommands("[]")).toEqual([]);
  });

  test("keeps saved order", () => {
    const saved = [command({ id: "b" }), command({ id: "a" })];
    expect(parseQuickCommands(JSON.stringify(saved))).toEqual(saved);
  });

  test("corrupt or off-contract data degrades to an empty list", () => {
    expect(parseQuickCommands("{not json")).toEqual([]);
    expect(parseQuickCommands(JSON.stringify([{ id: "x" }]))).toEqual([]);
  });

  test("duplicate IDs invalidate the list before keyed rendering", () => {
    const saved = [command(), command({ command: "git diff" })];
    expect(parseQuickCommands(JSON.stringify(saved))).toEqual([]);
  });
});

describe("labels", () => {
  test("unnamed commands use the first non-blank body line", () => {
    const multi = command({ command: "\n  first line\nsecond line" });
    expect(commandLabel(multi)).toBe("first line");
    expect(commandPreview(multi)).toBe("first line …");
    expect(commandLabel(command({ name: "状态" }))).toBe("状态");
    expect(commandPreview(command())).toBe("git status");
  });
});

describe("{{dir}}", () => {
  test("replaces every {{dir}} with the path verbatim", () => {
    expect(usesArtifactDir("看 {{dir}}")).toBe(true);
    expect(usesArtifactDir("git status")).toBe(false);
    expect(
      renderArtifactDir("看 {{dir}}，上传在 {{dir}}/uploads/", "/tmp/a b/'s1'"),
    ).toBe("看 /tmp/a b/'s1'，上传在 /tmp/a b/'s1'/uploads/");
  });

  test("the artifacts preset names the folder, uploads and cleanup", () => {
    const preset = QUICK_COMMAND_PRESETS.find(
      (cmd) => cmd.id === "preset-artifacts",
    );
    const text = renderArtifactDir(preset?.command ?? "", "/tmp/x/s1");
    expect(text).not.toContain("{{dir}}");
    expect(text).toContain("/tmp/x/s1/uploads/");
    expect(text).toContain("临时文件夹");
  });
});

describe("names", () => {
  test("requires a trimmed, short, unique name without braces", () => {
    expect(quickCommandNameError("  ", [])).not.toBe("");
    expect(quickCommandNameError("a{b", [])).not.toBe("");
    expect(quickCommandNameError("a\nb", [])).not.toBe("");
    expect(quickCommandNameError("n".repeat(41), [])).not.toBe("");
    expect(quickCommandNameError(" 状态 ", ["状态"])).not.toBe("");
    expect(quickCommandNameError(" 状态 ", ["其他"])).toBe("");
  });

  test("a list with a legacy empty name fails the save-time check only", () => {
    const legacy = [command({ id: "a" }), command({ id: "b", name: "x" })];
    expect(QuickCommandsSchema.safeParse(legacy).success).toBe(true);
    expect(quickCommandsNameError(legacy)).not.toBe("");
    expect(quickCommandsNameError(QUICK_COMMAND_PRESETS)).toBe("");
  });
});

describe("init instruction references", () => {
  const list = [
    command({ name: "状态", command: "git status {{dir}}" }),
    command({ id: "b", name: "套娃", command: "{{状态}}" }),
  ];

  test("lists referenced names once, in order", () => {
    expect(initInstructionReferences("{{a}} x {{ b }} {{a}} {c}")).toEqual([
      "a",
      "b",
    ]);
  });

  test("expands in a single pass and flags unknown names", () => {
    expect(
      expandInitInstruction("先 {{状态}}；{{套娃}}；{{无}}", list),
    ).toEqual({
      text: "先 git status {{dir}}；{{状态}}；{{无}}",
      missing: ["无"],
      hasUser: false,
      reserved: false,
    });
  });

  test("{{user}} is built in and {{webmux}} reserved; neither is a name", () => {
    expect(expandInitInstruction("任务：{{ user }}", list)).toEqual({
      text: "任务：{{user}}",
      missing: [],
      hasUser: true,
      reserved: false,
    });
    expect(expandInitInstruction("{{webmux}}", list)).toEqual({
      text: "{{webmux}}",
      missing: [],
      hasUser: false,
      reserved: true,
    });
    expect(quickCommandNameError("user", [])).not.toBe("");
    expect(quickCommandNameError("webmux", [])).not.toBe("");
    expect(expandInitInstruction("放 {{ dir }}", list)).toEqual({
      text: "放 {{dir}}",
      missing: [],
      hasUser: false,
      reserved: false,
    });
    expect(quickCommandNameError("dir", [])).not.toBe("");
  });
});
