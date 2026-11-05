<script lang="ts">
  import { rowStatusLabels, type RowStatus } from "../session-status";

  // The state at the end of a sidebar row: a coloured dot, right-aligned
  // with the row's other trailing marks. The row's accessible name and
  // tooltip carry the state's text, so it never depends on colour alone;
  // the dot keeps the text as its own tooltip too.
  let { status }: { status: RowStatus } = $props();
  const tones: Record<RowStatus, string> = {
    working: "bg-success",
    waiting: "bg-warning",
    idle: "bg-muted-foreground/70",
    ended: "border border-muted-foreground/70",
  };
</script>

<span
  class="flex size-4 shrink-0 items-center justify-end"
  title={rowStatusLabels[status]}
  data-status={status}
  ><span class={`size-2 rounded-full ${tones[status]}`} aria-hidden="true"
  ></span></span
>
