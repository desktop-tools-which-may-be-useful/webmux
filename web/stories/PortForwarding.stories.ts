import type { Meta, StoryObj } from "@storybook/svelte-vite";
import { userEvent, within } from "storybook/test";
import ForwardingHome from "./port-forwarding/ForwardingHome.svelte";

// The home page port-forwarding entry and dialog: the real WorkspaceHeader
// and ForwardDialog over demo forwards answered in memory.
const meta = {
  title: "Workspace/Port forwarding",
  component: ForwardingHome,
} satisfies Meta<typeof ForwardingHome>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Dialog: Story = { args: { dialog: true } };
export const Editing: Story = {
  args: { dialog: true },
  play: async ({ canvasElement }) => {
    // The dialog renders in a portal outside the story root.
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(
      await body.findByRole("button", { name: "端口 5173 的设置" }),
    );
  },
};
export const Empty: Story = { args: { dialog: true, empty: true } };
export const Unsupported: Story = {
  args: { dialog: true, empty: true, detection: false },
};
export const HeaderOnly: Story = { args: { dialog: false } };
