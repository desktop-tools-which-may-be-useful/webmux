<script lang="ts">
  import { Button } from "$lib/components/ui/button";

  // "终端 | 代码" on phones: both destinations stay visible and the pressed
  // one says where the session is, so the control never changes its label.
  let {
    first,
    code,
    onchange,
  }: {
    /** The pane shown beside the code view: the terminal or the chat. */
    first: string;
    code: boolean;
    onchange: (code: boolean) => void;
  } = $props();
</script>

<div
  class="inline-flex shrink-0 items-center gap-0.5 rounded-lg bg-muted p-0.5"
  role="group"
  aria-label="会话视图"
>
  <Button
    variant={code ? "quiet" : "default"}
    size="sm"
    aria-pressed={!code}
    onclick={() => onchange(false)}>{first}</Button
  >
  <Button
    variant={code ? "default" : "quiet"}
    size="sm"
    aria-pressed={code}
    onclick={() => onchange(true)}>代码</Button
  >
</div>
