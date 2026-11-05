<script lang="ts">
  import IconFile from "~icons/tabler/file";
  import IconPhoto from "~icons/tabler/photo";
  import IconX from "~icons/tabler/x";
  import { Button } from "$lib/components/ui/button";
  import type { ChatUploads } from "../../hooks/upload.svelte";

  // The attachments waiting to go with the next message, at the top of an
  // input card (the chat composer and quick start), each removable.
  let { uploads, disabled }: { uploads: ChatUploads; disabled: boolean } =
    $props();
</script>

{#if uploads.attachments.length}
  <ul class="flex flex-wrap gap-1 border-b px-2 py-1.5" aria-label="待发送附件">
    {#each uploads.attachments as item (item.path)}
      <li
        class="flex max-w-full min-w-0 items-center gap-1 rounded-sm border bg-muted/50 px-2 py-0.5 text-xs"
      >
        {#if item.kind === "image"}<IconPhoto
            class="size-3.5 shrink-0"
            aria-hidden="true"
          />{:else}<IconFile
            class="size-3.5 shrink-0"
            aria-hidden="true"
          />{/if}
        <span class="min-w-0 truncate">{item.name ?? item.path}</span>
        <Button
          variant="quiet"
          size="icon-xs"
          aria-label={`移除附件 ${item.name ?? item.path}`}
          title="移除附件"
          {disabled}
          onclick={() => uploads.remove(item.path)}
          ><IconX aria-hidden="true" /></Button
        >
      </li>
    {/each}
  </ul>
{/if}
