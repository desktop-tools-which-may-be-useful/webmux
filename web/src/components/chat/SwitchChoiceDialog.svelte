<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import type { ViewChangeChoice } from "../../schemas";

  // A switch to chat may need a policy choice before any interrupt, or after
  // an interrupt did not stop the TUI. The variants share one dialog shell,
  // but keep the two situations and their available actions distinct.
  let {
    open = $bindable(false),
    kind,
    note,
    busy,
    onchoose,
  }: {
    open?: boolean;
    kind: "ambiguous" | "timeout";
    note: string;
    busy: boolean;
    onchoose: (choice: ViewChangeChoice) => void;
  } = $props();
</script>

<AlertDialog.Root bind:open>
  <AlertDialog.Content>
    <AlertDialog.Header>
      {#if kind === "ambiguous"}
        <AlertDialog.Title>无法确定 TUI 是否正在执行回合</AlertDialog.Title>
        <AlertDialog.Description>
          WebMux 无法判断 TUI
          是停在基础输入界面，还是正在回合内等待你的输入。尚未发送中断键。
        </AlertDialog.Description>
      {:else}
        <AlertDialog.Title>TUI 仍在执行回合</AlertDialog.Title>
        <AlertDialog.Description>
          已发送中断键，10 秒内 TUI 仍未停下。
        </AlertDialog.Description>
      {/if}
    </AlertDialog.Header>
    {#if kind === "ambiguous"}
      <ul class="space-y-2 text-sm text-muted-foreground">
        <li>
          <strong class="text-foreground">取消</strong>：留在终端，TUI
          继续运行。
        </li>
        <li>
          <strong class="text-foreground">直接切换</strong
          >：不发送中断，直接结束 TUI。若 TUI
          正在回合内等待，未完成的回合可能丢失{note ? `，${note}` : ""}。
        </li>
        <li>
          <strong class="text-foreground">中断后切换</strong>：先发送中断键，等
          TUI 停下后再切换。
        </li>
      </ul>
    {:else}
      <ul class="space-y-2 text-sm text-muted-foreground">
        <li>
          <strong class="text-foreground">继续等待</strong>：再等待 10 秒。
        </li>
        <li>
          <strong class="text-foreground">强制切换</strong>：直接结束
          TUI。被强制结束的回合可能不完整{note ? `，${note}` : ""}。
        </li>
        <li>
          <strong class="text-foreground">取消</strong>：留在终端，TUI
          继续运行。
        </li>
      </ul>
    {/if}
    <AlertDialog.Footer>
      <Button
        variant="outline"
        disabled={busy}
        onclick={() => onchoose("cancel")}>取消</Button
      >
      <Button
        variant="destructive"
        disabled={busy}
        onclick={() => onchoose("force")}
        >{kind === "ambiguous" ? "直接切换" : "强制切换"}</Button
      >
      {#if kind === "ambiguous"}
        <Button disabled={busy} onclick={() => onchoose("interrupt")}
          >中断后切换</Button
        >
      {:else}
        <Button disabled={busy} onclick={() => onchoose("wait")}
          >继续等待</Button
        >
      {/if}
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
