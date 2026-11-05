<script lang="ts">
  import * as Command from "$lib/components/ui/command";

  // The searchable list of a repository's local branches, shared by the
  // new-worktree form's existing-branch field and quick start's base
  // branch. Rows are keyed by name (several branches can share a commit);
  // a row with a `reason` stays listed, disabled, with the reason under it.
  let {
    label,
    branches,
    selected,
    loading = false,
    failed = false,
    disabled = false,
    search = $bindable(""),
    onselect,
  }: {
    /** The list's accessible name. */
    label: string;
    branches: readonly { name: string; reason?: string }[];
    selected: string;
    loading?: boolean;
    /** The branch list could not be read. */
    failed?: boolean;
    disabled?: boolean;
    search?: string;
    onselect: (name: string) => void;
  } = $props();
</script>

<Command.Root
  {label}
  class="h-auto min-w-0"
  filter={(value, query) =>
    value.toLowerCase().includes(query.trim().toLowerCase()) ? 1 : 0}
  disableInitialScroll
>
  <Command.Input
    bind:value={search}
    placeholder="搜索本地分支…"
    aria-label="搜索本地分支"
    autocomplete="off"
    autocapitalize="off"
    autocorrect="off"
    spellcheck={false}
  />
  <Command.List aria-label="本地分支列表" class="max-h-[min(14rem,50dvh)] p-1">
    <Command.Empty>
      {loading
        ? "正在读取本地分支…"
        : failed
          ? "无法读取本地分支"
          : "没有匹配的本地分支"}
    </Command.Empty>
    {#each branches as item (item.name)}
      <Command.Item
        value={item.name}
        disabled={disabled || !!item.reason}
        data-checked={selected === item.name}
        onSelect={() => onselect(item.name)}
      >
        <span class="min-w-0 flex-1 break-all">
          {item.name}
          {#if item.reason}
            <span class="block text-muted-foreground">{item.reason}</span>
          {/if}
        </span>
      </Command.Item>
    {/each}
  </Command.List>
</Command.Root>
