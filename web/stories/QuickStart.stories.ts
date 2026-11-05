import type { Meta, StoryObj } from "@storybook/svelte-vite";
import QuickStart from "./quick-start/QuickStart.svelte";

// Prototype of the app shell and quick start (openspec change
// quick-start-chat). The mode buttons at the top left switch both the
// sidebar and the main area.
const meta = {
  title: "Workspace/Quick start",
  component: QuickStart,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof QuickStart>;

export default meta;
type Story = StoryObj<typeof meta>;

// Workbench mode: the repository tree and the existing overview.
export const Workbench: Story = { args: { mode: "workbench" } };
// A terminal session opened from the workbench tree: the tree stays, the
// terminal fills the main area.
export const WorkbenchTerminal: Story = {
  args: { mode: "workbench", session: "t2" },
};
// A chat-side hybrid session: opening it switched the shell to chat mode.
export const ChatSession: Story = { args: { mode: "chat", session: "s1" } };
// Chat mode: webmux has an init instruction; Claude Code's last reported
// options show.
export const Ready: Story = { args: { scenario: "ready" } };
// Codex never started a chat side here, so its options are not known yet.
export const NoOptionCatalog: Story = { args: { scenario: "no-options" } };
export const Sending: Story = { args: { scenario: "sending" } };
// The worktree exists, the agent did not start; the text stays.
export const AgentFailed: Story = { args: { scenario: "failed" } };
export const NoChatAgent: Story = { args: { scenario: "missing" } };
