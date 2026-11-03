<script lang="ts">
  import Brand from "./components/Brand.svelte";
  import { startSettings, stopSettings } from "./settings.svelte";
  import { errorMessage } from "./error";
  import { onMount, tick } from "svelte";
  import { api, consumeTokenFragment, navigate, store, stored } from "./api";
  import { HealthSchema } from "./schemas";
  import { sync as syncPush } from "./push";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Card from "$lib/components/ui/card";
  import TokenHelp from "./components/TokenHelp.svelte";
  import LazyPage from "./components/LazyPage.svelte";
  import SessionList from "./pages/SessionList.svelte";
  import { matchRoute } from "./route";
  import { createTerminalDrafts } from "./terminal-draft.svelte";
  import { browserDraftStorage } from "./draftStorage";
  import { clearHomeCache } from "./home-cache";

  const drafts = createTerminalDrafts({ storage: browserDraftStorage() });
  let uploadNotice = $state("");

  let path = $state(location.pathname);
  let ready = $state(false);
  let checking = $state(true);
  let token = $state(stored("token"));
  let error = $state("");
  const route = $derived(matchRoute(path));
  const sessionId = $derived(route.name === "terminal" ? route.id : "");
  // The frame of a pop-out code window; it only shows the code workspace.
  const codeId = $derived(route.name === "code" ? route.id : "");

  async function check() {
    checking = true;
    try {
      await api("/health", HealthSchema);
      ready = true;
      startSettings();
      error = "";
      // Re-registers an existing push subscription, e.g. after the server
      // rebuilt its store; without one it touches nothing.
      if (route.name !== "code") void syncPush().catch(() => undefined);
    } catch (err) {
      error = errorMessage(err);
    } finally {
      checking = false;
    }
  }

  onMount(() => {
    let focusRoute = false;
    const focusMain = () => {
      if (!focusRoute) return;
      const main = document.querySelector("main");
      if (!main) return;
      focusRoute = false;
      main.tabIndex = -1;
      main.focus({ preventScroll: true });
    };
    const routes = new MutationObserver(focusMain);
    routes.observe(document.getElementById("app")!, {
      childList: true,
      subtree: true,
    });
    const change = () => {
      path = location.pathname;
      focusRoute = true;
      void tick().then(focusMain);
    };
    const auth = () => {
      stopSettings();
      drafts.clearAll();
      clearHomeCache();
      uploadNotice = "";
      ready = false;
      checking = false;
      error = "Token 无效或已更改";
    };
    // Plain anchors to app routes would reload the whole document, which
    // blanks an iOS standalone PWA; route them through history instead.
    const link = (event: MouseEvent) => {
      if (
        event.defaultPrevented ||
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey
      )
        return;
      const anchor =
        event.target instanceof Element ? event.target.closest("a") : null;
      if (!anchor || anchor.hasAttribute("download")) return;
      if (anchor.target && anchor.target !== "_self") return;
      const url = new URL(anchor.href, location.href);
      if (url.origin !== location.origin) return;
      const appRoute =
        url.pathname === "/" || matchRoute(url.pathname).name !== "sessions";
      if (!appRoute) return;
      if (url.pathname === location.pathname && url.search === location.search)
        return;
      event.preventDefault();
      navigate(url.pathname + url.search + url.hash);
    };
    document.addEventListener("click", link);
    window.addEventListener("popstate", change);
    window.addEventListener("webmux:auth", auth);
    if (consumeTokenFragment()) token = stored("token");
    void check();
    return () => {
      stopSettings();
      routes.disconnect();
      drafts.clearAll();
      document.removeEventListener("click", link);
      window.removeEventListener("popstate", change);
      window.removeEventListener("webmux:auth", auth);
    };
  });
</script>

{#if !ready}
  <main class="flex min-h-dvh items-center justify-center p-6">
    <Card.Root class="w-full max-w-md">
      <Card.Header>
        <p class="brand mb-6"><Brand /></p>
        <Card.Title class="text-2xl">你的终端，始终在线。</Card.Title>
        <Card.Description>在可信内网中连接这台机器。</Card.Description>
      </Card.Header>
      <Card.Content>
        {#if checking}
          <p role="status" class="text-muted-foreground">正在连接…</p>
        {:else}
          <form
            class="space-y-5"
            onsubmit={(event) => {
              event.preventDefault();
              store("token", token);
              void check();
            }}
          >
            <div class="space-y-2">
              <div class="flex items-center gap-1">
                <Label for="token">共享 Token</Label>
                <TokenHelp />
              </div>
              <Input
                id="token"
                type="password"
                bind:value={token}
                autocomplete="current-password"
              />
            </div>
            {#if error}<p role="alert" class="text-destructive">{error}</p>{/if}
            <Button type="submit" class="w-full">连接 WebMux →</Button>
          </form>
        {/if}
      </Card.Content>
    </Card.Root>
  </main>
{:else if codeId}
  <LazyPage
    load={() => import("./pages/CodePopout.svelte")}
    label="代码"
    errorMessage={() => "代码窗口加载失败，请关闭后重新打开。"}
  >
    {#snippet children(page)}<page.default id={codeId} />{/snippet}
  </LazyPage>
{:else if sessionId}
  <LazyPage
    load={() => import("./pages/Terminal.svelte")}
    label="终端"
    errorMessage={(error) =>
      error instanceof Error ? error.message : "终端加载失败"}
  >
    {#snippet children(page)}
      {#key sessionId}<page.default
          id={sessionId}
          {drafts}
          onUploadNotice={(message) => {
            if (ready) uploadNotice = message;
          }}
        />{/key}
    {/snippet}
  </LazyPage>
{:else if route.name === "settings-tailscale"}
  <LazyPage
    load={() => import("./pages/TailscaleSettingsPage.svelte")}
    label="Tailscale 设置"
    loadingText="正在加载 Tailscale 设置…"
  >
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if route.name === "settings-tunnel"}
  <LazyPage
    load={() => import("./pages/TunnelSettingsPage.svelte")}
    label="公网访问设置"
  >
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if route.name === "settings"}
  <LazyPage load={() => import("./pages/Settings.svelte")} label="设置">
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else}
  <SessionList {drafts} />
{/if}

{#if ready && uploadNotice}
  <div
    class="fixed inset-x-2 top-2 z-50 flex items-start gap-2 rounded-md border bg-card p-3 text-sm shadow-lg"
    role="alert"
  >
    <p class="min-w-0 flex-1 break-all">{uploadNotice}</p>
    <Button variant="outline" size="sm" onclick={() => (uploadNotice = "")}
      >关闭</Button
    >
  </div>
{/if}
