<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import ResponsiveDialog from "../../src/components/ResponsiveDialog.svelte";

  // Prototype of the add-repository dialog: only the directory is asked; the
  // init instruction is edited later in the repository settings. Sample
  // values only; confirming reports what the server would register.
  let directory = $state("/home/developer/webmux");
  let open = $state(true);
  let notice = $state("");
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
        notice = `原型：将登记 ${directory}。`;
      }}
    >
      <div class="space-y-2">
        <Label for="prototype-repository">仓库目录</Label>
        <Input id="prototype-repository" bind:value={directory} required />
      </div>
      {#if notice}<p role="status" class="text-sm text-muted-foreground">
          {notice}
        </p>{/if}
      <div class="flex justify-end gap-2">
        <Button type="button" variant="outline" onclick={() => (open = false)}
          >取消</Button
        >
        <Button type="submit" disabled={!directory}>添加仓库</Button>
      </div>
    </form>
  </ResponsiveDialog>
{:else}
  <Button class="m-8" variant="outline" onclick={() => (open = true)}
    >打开“添加仓库”</Button
  >
{/if}
