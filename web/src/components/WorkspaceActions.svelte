<script lang="ts">
  import IconRefresh from "~icons/tabler/refresh";
  import IconSettings from "~icons/tabler/settings";
  import { Button } from "$lib/components/ui/button";
  import SessionViewControls from "./SessionViewControls.svelte";

  // The icon controls of the home page: list density, repository settings
  // and a Git refresh. Phones show them in the top bar, wider screens in the
  // toolbar, so the title and these controls never take two rows.
  let {
    compact,
    refreshing,
    onchange,
    onsettings,
    onrefresh,
  }: {
    compact: boolean;
    refreshing: boolean;
    onchange: (compact: boolean) => void;
    // Opens the settings of the repository in scope; absent when none is.
    onsettings?: () => void;
    onrefresh: () => void;
  } = $props();
</script>

<SessionViewControls {compact} {onchange} />
<Button
  variant="ghost"
  size="sm"
  class="min-h-9 sm:min-h-8"
  aria-label="仓库设置"
  title="仓库设置"
  disabled={!onsettings}
  onclick={() => onsettings?.()}
  ><IconSettings aria-hidden="true" /><span class="hidden sm:inline"
    >仓库设置</span
  ></Button
>
<Button
  variant="ghost"
  size="icon-sm"
  class="min-h-9 min-w-9 sm:min-h-8 sm:min-w-8"
  aria-label="刷新 Git 状态"
  title={refreshing ? "正在刷新 Git 状态…" : "刷新 Git 状态"}
  aria-busy={refreshing}
  onclick={() => onrefresh()}
  ><IconRefresh
    class={refreshing ? "animate-spin motion-reduce:animate-none" : ""}
    aria-hidden="true"
  /></Button
>
