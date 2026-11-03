<script lang="ts">
  import IconMessageCircle from "~icons/tabler/message-circle";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import * as Tabs from "$lib/components/ui/tabs";
  import { MediaQuery } from "svelte/reactivity";
  import type { SwitchTarget } from "../hybrid";

  // "聊天 | 终端" in a hybrid session's header: the selected tab is where the
  // session is. Narrow screens show only the icons, named for assistive
  // technology. A side that cannot be reached now stays focusable with its
  // reason; choosing it explains instead of switching.
  let {
    side,
    target,
    switching,
    chatUnavailable,
    request,
    code,
  }: {
    side: SwitchTarget | null;
    /** Where a running switch goes. */
    target: SwitchTarget | null;
    switching: boolean;
    chatUnavailable: string;
    request: (to: SwitchTarget) => void;
    /** The code view, as a third phone tab beside chat and terminal. */
    code?: { shown: boolean; set: (shown: boolean) => void };
  } = $props();
  const shown = $derived(target ?? side ?? "");
  // Phones fold the code view into the same switch: one control says where
  // the session is, whether that is chat, terminal or code.
  const phone = new MediaQuery("(max-width: 639px)");
  const labelled = $derived(phone.current && !!code);
  const inCode = $derived(phone.current && !!code?.shown);
</script>

<Tabs.Root
  bind:value={
    () => (inCode ? "code" : shown),
    (to) => {
      if (to === "code") code?.set(true);
      else if (to === "acp" || to === "tui") {
        if (inCode) code?.set(false);
        if (to !== shown) request(to);
      }
    }
  }
>
  <Tabs.List aria-label="会话视图">
    <Tabs.Trigger
      value="acp"
      disabled={switching}
      aria-disabled={chatUnavailable && !switching ? "true" : undefined}
      aria-label={chatUnavailable ? `聊天（${chatUnavailable}）` : "聊天"}
      title={chatUnavailable ? `现在不能切到聊天：${chatUnavailable}` : "聊天"}
      ><IconMessageCircle
        class={labelled ? "hidden" : ""}
        aria-hidden="true"
      /><span class={labelled ? "" : "max-sm:hidden"}>聊天</span></Tabs.Trigger
    >
    <Tabs.Trigger
      value="tui"
      disabled={switching}
      aria-label="终端"
      title="终端"
      ><IconTerminal2
        class={labelled ? "hidden" : ""}
        aria-hidden="true"
      /><span class={labelled ? "" : "max-sm:hidden"}>终端</span></Tabs.Trigger
    >
    {#if labelled}
      <Tabs.Trigger value="code" aria-label="代码" title="代码"
        >代码</Tabs.Trigger
      >
    {/if}
  </Tabs.List>
</Tabs.Root>
