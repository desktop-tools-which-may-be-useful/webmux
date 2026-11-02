import { expect, test } from "vitest";
import { choicesOf } from "../src/chat-config";
import type { ChatConfig } from "../src/schemas";

const option = (category: string): ChatConfig => ({
  id: category,
  name: category,
  category,
  type: "select",
  value: "anthropic/claude-haiku-4-5",
  options: [
    { value: "anthropic/claude-haiku-4-5", name: "Claude Haiku 4.5" },
    { value: "bedrock/claude-haiku-4-5", name: "Claude Haiku 4.5" },
  ],
});

test("OMP models are labelled with the full provider/model id", () => {
  expect(choicesOf("omp", option("model")).map((c) => c.name)).toEqual([
    "anthropic/claude-haiku-4-5",
    "bedrock/claude-haiku-4-5",
  ]);
});

test("other agents and other OMP options keep the agent's names", () => {
  expect(choicesOf("opencode", option("model"))[0]?.name).toBe(
    "Claude Haiku 4.5",
  );
  expect(choicesOf("omp", option("thought_level"))[0]?.name).toBe(
    "Claude Haiku 4.5",
  );
});
