<script lang="ts">
  import IconMessageCircle from "~icons/tabler/message-circle";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import * as Tabs from "$lib/components/ui/tabs";
  import { MediaQuery } from "svelte/reactivity";
  import type { SwitchTarget } from "../hybrid";

  // Where the session is, as one tab control. Phones always show it, text
  // only, and fold the code view in as the last tab: "终端 | 代码", or
  // "聊天 | 终端 | 代码" for a hybrid session. Wider screens keep the code
  // button apart and show only a hybrid session's "聊天 | 终端". A side that
  // cannot be reached now stays focusable with its reason; choosing it
  // explains instead of switching. While a switch runs or its target is
  // unknown, the tabs keep their places and none is selected.
  let {
    hybrid,
    pane,
    code,
  }: {
    /** A hybrid session's sides; absent for an ordinary session. */
    hybrid?: {
      side: SwitchTarget | null;
      /** Where a running switch goes. */
      target: SwitchTarget | null;
      switching: boolean;
      chatUnavailable: string;
      request: (to: SwitchTarget) => void;
    };
    /** The phone tab beside code when the session cannot switch sides. */
    pane: string;
    code: { shown: boolean; set: (shown: boolean) => void };
  } = $props();
  const phone = new MediaQuery("(max-width: 639px)");
  const inCode = $derived(phone.current && code.shown);
  const shown = $derived(
    hybrid ? (hybrid.target ?? hybrid.side ?? "") : "pane",
  );
  const unavailable = $derived(hybrid?.chatUnavailable ?? "");
  const switching = $derived(!!hybrid?.switching);
</script>

{#if hybrid || phone.current}
  <Tabs.Root
    bind:value={
      () => (inCode ? "code" : shown),
      (to) => {
        if (to === "code") return code.set(true);
        if (inCode) code.set(false);
        if ((to === "acp" || to === "tui") && to !== shown) hybrid?.request(to);
      }
    }
  >
    <Tabs.List variant="line" aria-label="会话视图">
      {#if hybrid}
        <Tabs.Trigger
          value="acp"
          disabled={switching}
          aria-disabled={unavailable && !switching ? "true" : undefined}
          aria-label={unavailable ? `聊天（${unavailable}）` : "聊天"}
          title={unavailable ? `现在不能切到聊天：${unavailable}` : "聊天"}
          ><IconMessageCircle
            class={phone.current ? "hidden" : ""}
            aria-hidden="true"
          /><span>聊天</span></Tabs.Trigger
        >
        <Tabs.Trigger
          value="tui"
          disabled={switching}
          aria-label="终端"
          title="终端"
          ><IconTerminal2
            class={phone.current ? "hidden" : ""}
            aria-hidden="true"
          /><span>终端</span></Tabs.Trigger
        >
      {:else}
        <Tabs.Trigger value="pane">{pane}</Tabs.Trigger>
      {/if}
      {#if phone.current}
        <Tabs.Trigger value="code">代码</Tabs.Trigger>
      {/if}
    </Tabs.List>
  </Tabs.Root>
{/if}
