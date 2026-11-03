<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { joinArgs } from "../api";
  import { LAUNCH_CONFIGS_LIMIT, launchConfigIcon } from "../launchConfigs";
  import type { LaunchConfigSummary, LaunchPreset } from "../schemas";
  import LaunchConfigIcon from "./LaunchConfigIcon.svelte";

  let {
    presets,
    configs,
    busy,
    onrestore,
  }: {
    presets: LaunchPreset[];
    configs: LaunchConfigSummary[];
    busy: boolean;
    onrestore: (name?: string) => void;
  } = $props();

  const existingNames = $derived(new Set(configs.map((config) => config.name)));
  const full = $derived(configs.length >= LAUNCH_CONFIGS_LIMIT);
  const missingCount = $derived(
    presets.filter((preset) => !existingNames.has(preset.name)).length,
  );
  const canRestoreAll = $derived(
    missingCount > 0 && configs.length + missingCount <= LAUNCH_CONFIGS_LIMIT,
  );
  const disabled = $derived(busy || full || presets.length === 0);
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger {disabled}>
    {#snippet child({ props })}
      <Button {...props} variant="outline" {disabled}>
        添加系统预设
        <IconChevronDown aria-hidden="true" />
      </Button>
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="start" class="w-80 max-w-[calc(100vw-1rem)]">
    <DropdownMenu.Item
      class="min-h-9 any-pointer-coarse:min-h-11"
      disabled={busy || !canRestoreAll}
      onSelect={() => onrestore()}
    >
      <span class="min-w-0 flex-1 truncate">补回全部缺失预设</span>
      {#if missingCount === 0}
        <Badge variant="outline">已齐全</Badge>
      {:else if !canRestoreAll}
        <Badge variant="outline">超出上限</Badge>
      {:else}
        <Badge variant="secondary">{missingCount} 项</Badge>
      {/if}
    </DropdownMenu.Item>
    <DropdownMenu.Separator />
    <div class="max-h-[min(55dvh,22rem)] overflow-x-hidden overflow-y-auto">
      {#each presets as preset (preset.name)}
        {@const exists = existingNames.has(preset.name)}
        <DropdownMenu.Item
          class="min-h-9 any-pointer-coarse:min-h-11"
          disabled={busy || exists || full}
          aria-label={exists
            ? `${preset.name}（已存在）`
            : `添加预设 ${preset.name}`}
          onSelect={() => onrestore(preset.name)}
        >
          <LaunchConfigIcon icon={launchConfigIcon(preset)} />
          <span class="flex min-w-0 flex-1 flex-col gap-0.5">
            <span class="flex min-w-0 items-center gap-1.5">
              <span class="truncate font-medium">{preset.name}</span>
              {#if preset.hybrid}<Badge variant="secondary">混合</Badge>{/if}
              {#if preset.envNames.length}<Badge variant="secondary"
                  >{preset.envNames.length} 个环境变量</Badge
                >{/if}
            </span>
            <span class="truncate font-mono text-xs text-muted-foreground"
              >{joinArgs(preset.cmd)}</span
            >
          </span>
          {#if exists}
            <Badge variant="outline">已存在</Badge>
          {/if}
        </DropdownMenu.Item>
      {/each}
    </div>
  </DropdownMenu.Content>
</DropdownMenu.Root>
