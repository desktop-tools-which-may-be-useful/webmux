<script lang="ts">
  import { statusLabels, type ChatStatus } from "./data";

  // The status shown at the end of a sidebar row: colour plus text, so the
  // state never depends on colour alone.
  let { status }: { status: ChatStatus } = $props();
  const tones: Record<ChatStatus, string> = {
    working: "text-success",
    waiting: "text-warning",
    idle: "text-muted-foreground",
    ended: "text-muted-foreground",
  };
</script>

<span class={`shrink-0 text-xs font-normal ${tones[status]}`}
  >{statusLabels[status]}</span
>
