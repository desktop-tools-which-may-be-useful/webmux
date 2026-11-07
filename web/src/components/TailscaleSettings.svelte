<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as Card from "$lib/components/ui/card";
  import TunnelStateBadge from "./TunnelStateBadge.svelte";
  import { useMountedRead } from "../hooks/remote-request.svelte";
  import { TailscaleStatusSchema } from "../schemas";

  const read = useMountedRead("/tailscale", TailscaleStatusSchema);
  const status = $derived(read.value);
  const error = $derived(read.error);
  const modeText = {
    off: "未启用",
    serve: "Serve（仅 tailnet）",
    funnel: "Funnel（公网）",
  };
</script>

<Card.Root flat id="tailscale" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>Tailscale（Serve / Funnel）</Card.Title>
    <Card.Description
      >使用 Tailscale 自动签发的 HTTPS 证书。Serve 仅 tailnet
      内设备可访问；Funnel 会公开到互联网。可在网页修改，保存后立即生效。</Card.Description
    >
  </Card.Header>
  <Card.Content class="space-y-4">
    {#if error}
      <p role="alert" class="text-sm text-destructive">{error}</p>
    {:else if status}
      <div class="flex flex-wrap items-center gap-2" role="status">
        <TunnelStateBadge state={status.state} />
        <span class="text-sm">{modeText[status.mode]}</span>
        {#if status.dnsName}
          <span class="text-sm text-muted-foreground">{status.dnsName}</span>
        {/if}
      </div>
      {#if status.publicUrl}
        <p class="text-sm wrap-anywhere">{status.publicUrl}</p>
      {:else if status.blockedReason || status.error}
        <p class="text-sm text-muted-foreground wrap-anywhere">
          {status.blockedReason || status.error}
        </p>
      {:else}
        <p class="text-sm text-muted-foreground">
          尚未启用；进入 Tailscale 设置选择 Serve 或 Funnel。
        </p>
      {/if}
    {:else}
      <p class="text-sm text-muted-foreground">正在读取 Tailscale 状态…</p>
    {/if}
    <Button href="/settings/tailscale" variant="outline">管理 Tailscale</Button>
  </Card.Content>
</Card.Root>
