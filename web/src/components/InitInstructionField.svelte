<script lang="ts">
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import { INIT_INSTRUCTION_MAX } from "../schemas";

  // The repository init instruction: an optional prefix quick start sends
  // before the first message. The counter turns red past the server limit.
  let {
    value = $bindable(),
    id,
    label = "初始化指令（可选）",
  }: { value: string; id: string; label?: string } = $props();
  const over = $derived(value.length > INIT_INSTRUCTION_MAX);
</script>

<div class="space-y-2">
  <Label for={id}>{label}</Label>
  <Textarea
    {id}
    rows={4}
    bind:value
    aria-invalid={over}
    aria-describedby={`${id}-help`}
    placeholder="例如：先阅读 AGENTS.md，改完运行 make check"
  />
  <p id={`${id}-help`} class="flex gap-2 text-xs text-muted-foreground">
    <span class="mr-auto"
      >快速开始时放在你的消息前面，一起作为第一条消息发送。</span
    >
    <span class={`tabular-nums ${over ? "text-destructive" : ""}`}
      >{value.length} / {INIT_INSTRUCTION_MAX}</span
    >
  </p>
</div>
