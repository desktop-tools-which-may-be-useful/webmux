<script lang="ts">
  import { errorMessage } from "../error";
  import { api } from "../api";
  import {
    INIT_INSTRUCTION_MAX,
    RegisterRepositoryRequestSchema,
    RepositorySchema,
  } from "../schemas";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import DirectoryPicker from "./DirectoryPicker.svelte";
  import InitInstructionField from "./InitInstructionField.svelte";
  import ResponsiveDialog from "./ResponsiveDialog.svelte";
  let {
    defaultCwd,
    onclose,
    onchange,
  }: { defaultCwd: string; onclose: () => void; onchange: () => void } =
    $props();
  let path = $state("");
  let instruction = $state("");
  let busy = $state(false);
  let error = $state("");
  // The repository was registered before and kept its own instruction.
  let kept = $state("");
  async function add() {
    busy = true;
    error = "";
    kept = "";
    const initInstruction = instruction.trim();
    try {
      const repository = await api(
        "/repositories",
        RepositorySchema,
        "POST",
        { path, ...(initInstruction ? { initInstruction } : {}) },
        undefined,
        RegisterRepositoryRequestSchema,
      );
      onchange();
      // Registering again never replaces an instruction; say so instead of
      // closing as if this one had been saved.
      if (initInstruction && repository.initInstruction !== initInstruction) {
        kept = repository.name;
        return;
      }
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<ResponsiveDialog
  title="添加仓库"
  description="选择已有且可信的本机 Git 仓库。主目录、worktree 或子目录均可；同一仓库只登记一次，不会 init 或 clone。"
  {onclose}
  {busy}
>
  <form
    class="space-y-4"
    onsubmit={(event) => {
      event.preventDefault();
      void add();
    }}
  >
    <div class="space-y-2">
      <Label for="repository-path">仓库目录</Label>
      <div class="flex gap-2">
        <Input
          id="repository-path"
          bind:value={path}
          placeholder={defaultCwd}
          required
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
        />
        <DirectoryPicker bind:value={path} />
      </div>
    </div>
    <InitInstructionField
      id="repository-instruction"
      bind:value={instruction}
    />
    <p class="text-sm text-muted-foreground">
      不是 Git 仓库？无需添加，使用「新建会话 → 直接目录」即可。
    </p>
    {#if kept}<p role="status" class="text-sm text-warning">
        {kept} 已经登记过，原有的初始化指令没有改变。请在仓库菜单的“初始化指令…”中修改。
      </p>{/if}
    {#if error}<p role="alert" class="break-all text-sm text-destructive">
        {error}
      </p>{/if}
    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline" disabled={busy} onclick={onclose}
        >{kept ? "关闭" : "取消"}</Button
      >
      <Button
        type="submit"
        disabled={busy || !path || instruction.length > INIT_INSTRUCTION_MAX}
        >{busy ? "正在添加…" : "添加仓库"}</Button
      >
    </div>
  </form>
</ResponsiveDialog>
