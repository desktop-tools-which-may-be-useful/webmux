<script lang="ts">
  import type { Snippet } from "svelte";
  import type { HTMLAttributes } from "svelte/elements";
  import type { ChatUploads } from "../hooks/upload.svelte";

  // An area that takes files dropped on it as attachments of the next
  // message (the chat view and quick start), with a dashed overlay while
  // files hover. Other drags pass through untouched.
  let {
    uploads,
    blocked = false,
    label,
    children,
    ...rest
  }: {
    uploads: ChatUploads;
    /** Uploads are refused now; a drop does nothing. */
    blocked?: boolean;
    /** The overlay's text while files hover. */
    label: string;
    children: Snippet;
  } & Omit<HTMLAttributes<HTMLDivElement>, "children"> = $props();
  let dragging = $state(false);
  let depth = 0;

  const hasFiles = (event: DragEvent) =>
    event.dataTransfer?.types.includes("Files") ?? false;
  function dragEnter(event: DragEvent) {
    if (!hasFiles(event)) return;
    event.preventDefault();
    if (blocked) return;
    dragging = true;
    depth++;
  }
  function dragOver(event: DragEvent) {
    if (!hasFiles(event)) return;
    event.preventDefault();
    if (event.dataTransfer)
      event.dataTransfer.dropEffect =
        blocked || uploads.progress ? "none" : "copy";
  }
  function dragLeave() {
    if (--depth <= 0) {
      depth = 0;
      dragging = false;
    }
  }
  function drop(event: DragEvent) {
    if (!hasFiles(event)) return;
    event.preventDefault();
    depth = 0;
    dragging = false;
    if (blocked) return;
    void uploads.uploadFiles(Array.from(event.dataTransfer?.files ?? []));
  }
</script>

<div
  {...rest}
  ondragenter={dragEnter}
  ondragover={dragOver}
  ondragleave={dragLeave}
  ondrop={drop}
>
  {#if dragging}
    <div
      class="pointer-events-none absolute inset-2 z-20 flex items-center justify-center rounded-lg border-2 border-dashed border-ring bg-background/85 text-sm"
      role="status"
    >
      {uploads.progress ? "请等待当前上传完成" : label}
    </div>
  {/if}
  {@render children()}
</div>
