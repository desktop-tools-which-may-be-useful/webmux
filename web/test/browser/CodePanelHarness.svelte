<script lang="ts">
  import CodePanel from "../../src/components/CodePanel.svelte";
  let visible = $state(true);
  let inserted = $state("");
  let reveal = $state<{ path: string; key: number }>();
</script>

<button onclick={() => (visible = !visible)}>Toggle code visibility</button>
<button
  onclick={() =>
    (reveal = { path: "/root/file-3.txt", key: (reveal?.key ?? 0) + 1 })}
  >Reveal file 3</button
>
<div style="height:650px;display:flex;flex-direction:column">
  <CodePanel
    id="code-session"
    {visible}
    {reveal}
    connection="connected"
    draftEnabled={false}
    paste={(text) => {
      inserted = text;
      return "terminal";
    }}
  />
</div>
<output aria-label="Inserted reference">{inserted}</output>
