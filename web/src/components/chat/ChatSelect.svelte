<script lang="ts">
  import type { Snippet } from "svelte";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconSettings from "~icons/tabler/settings";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

  // A compact select below the chat input: a quiet button showing the
  // option's icon that opens a radio menu. The value comes from the server
  // state; the accessible name and tooltip keep the full name and current
  // value, and a pick only reports the choice. `labeled` also shows the
  // current value's text, for choices that must read at a glance (quick
  // start's repository, base branch and agent); a choice with a `reason`
  // stays in the menu, disabled, with the reason under it.
  let {
    name,
    value,
    choices,
    disabled,
    title,
    labeled = false,
    labelClass = "",
    icon,
    choiceIcon,
    onchange,
  }: {
    name: string;
    value: string;
    choices: { value: string; name: string; reason?: string }[];
    disabled: boolean;
    title?: string;
    labeled?: boolean;
    /** Extra classes on the value text, e.g. to hide it in a narrow card. */
    labelClass?: string;
    icon?: Snippet;
    /** A per-choice icon, shown in the menu and, when labeled, the button. */
    choiceIcon?: Snippet<[string]>;
    onchange: (value: string) => void;
  } = $props();
  const current = $derived(
    choices.find((choice) => choice.value === value)?.name ?? value,
  );
  const label = $derived(`${name}：${current}`);
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger {disabled}>
    {#snippet child({ props })}
      <Button
        {...props}
        variant="quiet"
        size="xs"
        class={[
          // Option icons read at the same 16px as the neighbouring icon buttons.
          "[&_svg:not([class*='size-'])]:size-4",
          labeled ? "max-w-48 min-w-0 shrink" : "shrink-0",
        ]}
        aria-label={label}
        title={title ?? label}
      >
        {#if labeled && choiceIcon}{@render choiceIcon(
            value,
          )}{:else if icon}{@render icon()}{:else}<IconSettings
            aria-hidden="true"
          />{/if}
        {#if labeled}<span class={["min-w-0 truncate", labelClass]}
            >{current}</span
          >{/if}
        <IconChevronDown class="size-3" aria-hidden="true" />
      </Button>
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="start" class="max-h-80 min-w-52">
    <DropdownMenu.Label>{name}</DropdownMenu.Label>
    <DropdownMenu.RadioGroup {value} onValueChange={onchange}>
      {#each choices as choice (choice.value)}
        <DropdownMenu.RadioItem
          value={choice.value}
          disabled={disabled || !!choice.reason}
        >
          {#if choiceIcon}{@render choiceIcon(choice.value)}{/if}
          <span class="min-w-0">
            {choice.name}
            {#if choice.reason}<span class="block text-xs text-muted-foreground"
                >{choice.reason}</span
              >{/if}
          </span>
        </DropdownMenu.RadioItem>
      {/each}
    </DropdownMenu.RadioGroup>
  </DropdownMenu.Content>
</DropdownMenu.Root>
