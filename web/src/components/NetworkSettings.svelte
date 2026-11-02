<script lang="ts">
  import * as Card from "$lib/components/ui/card";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import { setWsCompression, wsCompression } from "../ws";

  let compression = $state(wsCompression());
</script>

<Card.Root id="network" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>网络</Card.Title>
    <Card.Description
      >仅对当前浏览器生效。移动网络等慢速链路上开启压缩可减少终端回放与输出的流量；局域网或本机访问通常无需开启。</Card.Description
    >
  </Card.Header>
  <Card.Content class="space-y-2">
    <div class="flex items-center justify-between gap-4">
      <Label for="ws-compression">压缩实时连接（permessage-deflate）</Label>
      <Switch
        id="ws-compression"
        checked={compression}
        aria-describedby="ws-compression-help"
        onCheckedChange={(checked) => {
          setWsCompression(checked);
          compression = wsCompression();
        }}
      />
    </div>
    <p id="ws-compression-help" class="text-xs text-muted-foreground">
      切换后已打开的终端与列表连接会立即重连，输出从断点续传。小于 128
      字节的消息（如按键回显）不压缩，不增加输入延迟；每个连接在服务端多占约 1.3
      MB 内存。
    </p>
  </Card.Content>
</Card.Root>
