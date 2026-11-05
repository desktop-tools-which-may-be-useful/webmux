import type { Meta, StoryObj } from "@storybook/svelte-vite";
import { userEvent, within } from "storybook/test";
import PortPrompt from "./port-forwarding/PortPrompt.svelte";

// The terminal page prompts for listening ports: the real TerminalNotices
// over demo forwards answered in memory.
const meta = {
  title: "Terminal/Port prompt",
  component: PortPrompt,
} satisfies Meta<typeof PortPrompt>;

export default meta;
type Story = StoryObj<typeof meta>;

async function forward5173(canvasElement: HTMLElement) {
  await userEvent.click(
    await within(canvasElement).findByRole("button", { name: "转发" }),
  );
}

export const NewPort: Story = {};
export const MultiplePorts: Story = { args: { multiple: true } };
export const Forwarded: Story = {
  play: ({ canvasElement }) => forward5173(canvasElement),
};
export const Failed: Story = {
  args: { failing: true },
  play: ({ canvasElement }) => forward5173(canvasElement),
};
