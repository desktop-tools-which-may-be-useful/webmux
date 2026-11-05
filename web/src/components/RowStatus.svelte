<script lang="ts">
  import { rowStatusLabels, type RowStatus } from "../session-status";

  // The state at the end of a sidebar row: colour plus text, so it never
  // depends on colour alone.
  let { status }: { status: RowStatus } = $props();
  const tones: Record<RowStatus, string> = {
    working: "text-success",
    waiting: "text-warning",
    idle: "text-muted-foreground",
    ended: "text-muted-foreground",
  };
</script>

<!-- A fixed width keeps the row still when the state text changes. -->
<span class={`w-10 shrink-0 text-xs font-normal ${tones[status]}`}
  >{rowStatusLabels[status]}</span
>
