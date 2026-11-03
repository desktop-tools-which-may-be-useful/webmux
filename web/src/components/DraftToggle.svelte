<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import type { TerminalDraft } from "../terminal-draft.svelte";

  // The draft-mode switch. Wide screens keep it in the header; phones keep it
  // beside the soft keys, where input actions live.
  let {
    draft,
    draftSaved,
    scratchSaved,
    disabled,
    toggle,
    class: className,
  }: {
    draft: TerminalDraft;
    draftSaved: string;
    scratchSaved: string;
    disabled: boolean;
    toggle: () => void;
    class?: string;
  } = $props();
</script>

<Button
  variant={draft.enabled ? "default" : "outline"}
  size="sm"
  class={className}
  aria-label={draft.scratch.length
    ? `草稿模式，${draft.scratch.length} 条暂存`
    : "草稿模式"}
  aria-pressed={draft.enabled}
  {disabled}
  title={draft.enabled
    ? "关闭只隐藏草稿，不发送或清空"
    : draft.scratchUpdatedWhileHidden
      ? `暂存已更新，开启查看；${scratchSaved}`
      : draft.scratch.length
        ? `开启查看草稿与 ${draft.scratch.length} 条暂存；${scratchSaved}`
        : draft.updatedWhileHidden
          ? `隐藏草稿已更新，开启查看；${draftSaved}`
          : draft.text
            ? `开启查看保留的草稿；${draftSaved}`
            : "开启草稿模式"}
  onclick={toggle}
  >草稿{#if !draft.enabled && draft.scratch.length}
    ·{draft.scratch
      .length}{:else if !draft.enabled && (draft.text || draft.updatedWhileHidden)}
    ·{/if}</Button
>
