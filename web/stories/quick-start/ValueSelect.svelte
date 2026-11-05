<script lang="ts">
  import type { Snippet } from "svelte";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

  // Prototype of the labelled variant of ChatSelect: the quiet button shows
  // the current value's text beside its icon, so the repository, base branch
  // and agent read at a glance under the input. Unavailable choices stay in
  // the menu, disabled, with their reason.
  let {
    name,
    value,
    choices,
    disabled = false,
    icon,
    onchange,
  }: {
    name: string;
    value: string;
    choices: { value: string; name: string; reason?: string }[];
    disabled?: boolean;
    icon?: Snippet<[string]>;
    onchange: (value: string) => void;
  } = $props();
  const current = $derived(
    choices.find((choice) => choice.value === value)?.name ?? value,
  );
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger {disabled}>
    {#snippet child({ props })}
      <Button
        {...props}
        variant="quiet"
        size="xs"
        class="max-w-48 min-w-0 shrink-0"
        aria-label={`${name}：${current}`}
        title={`${name}：${current}`}
      >
        {#if icon}{@render icon(value)}{/if}
        <span class="min-w-0 truncate">{current}</span>
        <IconChevronDown aria-hidden="true" />
      </Button>
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="start" class="min-w-56">
    <DropdownMenu.Label>{name}</DropdownMenu.Label>
    <DropdownMenu.RadioGroup {value} onValueChange={onchange}>
      {#each choices as choice (choice.value)}
        <DropdownMenu.RadioItem value={choice.value} disabled={!!choice.reason}>
          {#if icon}{@render icon(choice.value)}{/if}
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
