import type { Meta, StoryObj } from "@storybook/svelte-vite";
import AddRepository from "./quick-start/AddRepository.svelte";

// The add-repository dialog with the init instruction (quick-start-chat).
const meta = {
  title: "Workspace/Quick start/Add repository",
  component: AddRepository,
} satisfies Meta<typeof AddRepository>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WithInstruction: Story = { args: { filled: true } };
export const Empty: Story = { args: { filled: false } };
