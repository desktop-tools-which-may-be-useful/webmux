<script lang="ts">
  import { untrack } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import ResponsiveDialog from "../../src/components/ResponsiveDialog.svelte";

  // Prototype of the add-repository dialog with the optional init
  // instruction that quick start puts before the first message. Sample
  // values only; confirming reports what the server would store.
  let { filled = true }: { filled?: boolean } = $props();
  const limit = 4000;
  let directory = $state("/home/developer/webmux");
  let instruction = $state(
    untrack(() => filled)
      ? "先阅读 AGENTS.md 和相关 docs/agent-rules，改完运行 make check；用 Conventional Commits 提交并开 draft PR。"
      : "",
  );
  let open = $state(true);
  let notice = $state("");
  const over = $derived(instruction.length > limit);
</script>

{#if open}
  <ResponsiveDialog
    title="添加仓库"
    description="选择已有且可信的本机 Git 仓库。主目录、worktree 或子目录均可；同一仓库只登记一次，不会 init 或 clone。"
    onclose={() => (open = false)}
  >
    <form
      class="space-y-4"
      onsubmit={(event) => {
        event.preventDefault();
        notice = `原型：将登记 ${directory}${instruction.trim() ? "，并保存初始化指令" : ""}。`;
      }}
    >
      <div class="space-y-2">
        <Label for="prototype-repository">仓库目录</Label>
        <Input id="prototype-repository" bind:value={directory} required />
      </div>
      <div class="space-y-2">
        <Label for="prototype-instruction">初始化指令（可选）</Label>
        <Textarea
          id="prototype-instruction"
          rows={4}
          bind:value={instruction}
          aria-invalid={over}
          aria-describedby="prototype-instruction-help"
          placeholder="例如：先阅读 AGENTS.md，改完运行 make check"
        />
        <p
          id="prototype-instruction-help"
          class="flex gap-2 text-xs text-muted-foreground"
        >
          <span class="mr-auto"
            >快速开始时放在你的消息前面，一起作为第一条消息发送。</span
          >
          <span class={over ? "text-destructive" : ""}
            >{instruction.length} / {limit}</span
          >
        </p>
      </div>
      {#if notice}<p role="status" class="text-sm text-muted-foreground">
          {notice}
        </p>{/if}
      <div class="flex justify-end gap-2">
        <Button type="button" variant="outline" onclick={() => (open = false)}
          >取消</Button
        >
        <Button type="submit" disabled={!directory || over}>添加仓库</Button>
      </div>
    </form>
  </ResponsiveDialog>
{:else}
  <Button class="m-8" variant="outline" onclick={() => (open = true)}
    >打开“添加仓库”</Button
  >
{/if}
