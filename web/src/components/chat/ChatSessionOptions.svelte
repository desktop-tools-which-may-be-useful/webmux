<script lang="ts">
  import IconSettings from "~icons/tabler/settings";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import * as Popover from "$lib/components/ui/popover";
  import { Switch } from "$lib/components/ui/switch";
  import type { ConfigValue } from "../../chat-api";
  import {
    ChatPermissionModeSchema,
    type ChatConfig,
    type ChatPermissionMode,
  } from "../../schemas";

  // Narrow screens keep the session options out of the bottom bar: one button
  // beside the attachment control opens them as a list. The set comes from the
  // agent, so the list is built from whatever options the session reports.
  let {
    mode,
    config,
    disabled,
    modeDisabled,
    onmodechange,
    onconfigchange,
  }: {
    mode: ChatPermissionMode;
    config: ChatConfig[];
    disabled: boolean;
    modeDisabled: boolean;
    onmodechange: (mode: ChatPermissionMode) => void;
    onconfigchange: (configId: string, value: ConfigValue) => void;
  } = $props();
  const modes = [
    { value: "ask", name: "每次询问" },
    { value: "auto", name: "自动批准" },
  ];
</script>

<Popover.Root>
  <Popover.Trigger>
    {#snippet child({ props })}
      <Button
        {...props}
        variant="ghost"
        size="icon-touch"
        aria-label="会话选项"
        title="会话选项"><IconSettings aria-hidden="true" /></Button
      >
    {/snippet}
  </Popover.Trigger>
  <Popover.Content align="start" side="top" class="w-[min(20rem,92vw)]">
    <Label class="flex-col items-stretch">
      权限
      <NativeSelect.Root
        aria-label="权限"
        disabled={modeDisabled}
        value={mode}
        onchange={(event) => {
          const parsed = ChatPermissionModeSchema.safeParse(
            event.currentTarget.value,
          );
          if (parsed.success && parsed.data !== mode) onmodechange(parsed.data);
        }}
      >
        {#each modes as choice (choice.value)}
          <NativeSelect.Option value={choice.value}
            >{choice.name}</NativeSelect.Option
          >
        {/each}
      </NativeSelect.Root>
    </Label>
    {#each config as option (option.id)}
      {#if option.type === "boolean"}
        <Label class="justify-between">
          {option.name}
          <Switch
            checked={!!option.enabled}
            {disabled}
            onCheckedChange={(enabled) =>
              onconfigchange(option.id, { enabled })}
          />
        </Label>
      {:else}
        <Label class="flex-col items-stretch">
          {option.name}
          <NativeSelect.Root
            aria-label={option.name}
            {disabled}
            value={option.value ?? ""}
            onchange={(event) =>
              onconfigchange(option.id, { value: event.currentTarget.value })}
          >
            {#each option.options as choice (choice.value)}
              <NativeSelect.Option value={choice.value}
                >{choice.name}</NativeSelect.Option
              >
            {/each}
          </NativeSelect.Root>
        </Label>
      {/if}
    {/each}
  </Popover.Content>
</Popover.Root>
