<script lang="ts">
  import type { Snippet } from "svelte";
  import IconArrowDown from "~icons/tabler/arrow-down";
  import {
    Button,
    type ButtonSize,
    type ButtonVariant,
  } from "$lib/components/ui/button";
  import SoftKeys from "./SoftKeys.svelte";
  import TouchGamepad from "./TouchGamepad.svelte";
  import type { Modifiers } from "../input";
  import type { UploadProgress } from "../hooks/upload.svelte";
  import type { Connection } from "../ws";
  import { hasTouchInput } from "../touch-input";

  interface ViewportTerminal {
    readonly selection: string;
    readonly restoring: boolean;
    readonly atBottom: boolean;
    readonly connection: Connection;
    readonly controlling?: boolean;
    readonly exitCode?: number | null;
    readonly modifiers: Modifiers;
    readonly shiftLocked: boolean;
    copySelection(): void;
    clearSelection(): void;
    scrollToBottom(): void;
  }
  interface ViewportUploads {
    readonly dragging: boolean;
    readonly progress: UploadProgress | null;
  }

  // The xterm host with its overlays, above the soft-key toolbar.
  let {
    container = $bindable(),
    terminal: t,
    visible,
    sizeLocked = false,
    uploads,
    quickMenu,
    draftToggle,
    send,
    toggle,
    lock,
    paste,
    keyboard,
    touchGamepad = false,
    findOpen = false,
    findBar,
  }: {
    container?: HTMLDivElement;
    terminal: ViewportTerminal;
    // Whether the terminal pane is on screen (mobile may show code instead).
    visible: boolean;
    // Keeps the PTY grid fixed and lets the host scroll past its edges.
    sizeLocked?: boolean;
    uploads: ViewportUploads;
    quickMenu: Snippet<["top" | "bottom", ButtonSize?, ButtonVariant?]>;
    // Phones keep the draft switch beside the quick commands.
    draftToggle: Snippet;
    send: (input: string) => void;
    toggle: (key: keyof Modifiers) => void;
    lock: (key: "shift") => void;
    paste: () => Promise<void>;
    keyboard: () => void;
    touchGamepad?: boolean;
    findOpen?: boolean;
    findBar?: Snippet;
  } = $props();

  const gamepadActive = $derived(
    touchGamepad &&
      t.controlling === true &&
      t.connection !== "missing" &&
      (t.exitCode === null || t.exitCode === undefined) &&
      visible,
  );
  let gamepadFaded = $state(false);
  $effect(() => {
    const host = container;
    if (!host || !gamepadActive) {
      gamepadFaded = false;
      return;
    }
    let fadeTimer: ReturnType<typeof setTimeout> | undefined;
    const onHostPointerDown = () => {
      clearTimeout(fadeTimer);
      gamepadFaded = true;
    };
    const onHostPointerEnd = () => {
      clearTimeout(fadeTimer);
      fadeTimer = setTimeout(() => {
        gamepadFaded = false;
      }, 600);
    };
    host.addEventListener("pointerdown", onHostPointerDown, true);
    host.addEventListener("pointerup", onHostPointerEnd, true);
    host.addEventListener("pointercancel", onHostPointerEnd, true);
    return () => {
      clearTimeout(fadeTimer);
      host.removeEventListener("pointerdown", onHostPointerDown, true);
      host.removeEventListener("pointerup", onHostPointerEnd, true);
      host.removeEventListener("pointercancel", onHostPointerEnd, true);
    };
  });

  // Keep the hint off touch screens, including hybrids whose secondary pointer
  // is coarse, and mirror the platform's actual copy shortcut.
  const mac = /Mac/.test(navigator.platform);
  const shortcutHint =
    !hasTouchInput() && matchMedia("(pointer: fine)").matches;
</script>

<div class="terminal-column">
  <div class="terminal-area">
    <div
      class="terminal-host"
      class:size-locked={sizeLocked}
      class:restoring={t.restoring && t.connection !== "missing"}
      bind:this={container}
    ></div>
    {@render findBar?.()}
    {#if uploads.dragging}
      <div
        class="pointer-events-none absolute inset-0 z-20 flex items-center justify-center border-2 border-dashed border-primary bg-background/90"
        role="status"
      >
        <p class="text-sm">
          {uploads.progress ? "请等待当前上传完成" : "松开以上传文件到临时目录"}
        </p>
      </div>
    {/if}
    {#if t.selection && !findOpen}
      <div
        class="absolute right-2 top-2 z-10 flex items-center gap-1 rounded-md border bg-popover p-1 shadow-md"
        role="toolbar"
        aria-label="终端文字选择"
      >
        <Button
          size="sm"
          aria-keyshortcuts={mac ? "Meta+C" : "Control+C"}
          onclick={() => t.copySelection()}>复制选中</Button
        >
        {#if shortcutHint}<kbd class="border px-1" aria-hidden="true"
            >{mac ? "⌘C" : "Ctrl+C"}</kbd
          >{/if}
        <Button variant="ghost" size="sm" onclick={() => t.clearSelection()}
          >取消选择</Button
        >
      </div>
    {/if}
    {#if t.restoring && t.connection !== "missing"}<div
        class="restore-overlay"
        role="status"
      >
        {t.connection === "connecting" ? "正在连接…" : "正在恢复…"}
      </div>{:else if t.connection === "connecting" || t.connection === "reconnecting"}<div
        class="pointer-events-none absolute top-2 right-2 z-20"
      >
        <span class="text-xs text-muted-foreground" role="status"
          >正在恢复…</span
        >
      </div>{/if}
    {#if !t.atBottom && !t.restoring && visible}<Button
        variant="outline"
        size="sm"
        raised
        class={gamepadActive
          ? "absolute right-3 bottom-36 z-10 sm:hidden"
          : "absolute right-3 bottom-3 z-10 sm:hidden"}
        aria-label="回到最新"
        onmousedown={(event) => event.preventDefault()}
        onclick={() => t.scrollToBottom()}
        ><IconArrowDown aria-hidden="true" />回到最新</Button
      >{/if}
    {#if gamepadActive}
      <TouchGamepad
        {send}
        faded={gamepadFaded}
        oninteract={() => {
          gamepadFaded = false;
        }}
      />
    {/if}
  </div>
  <SoftKeys
    {send}
    modifiers={t.modifiers}
    locked={t.shiftLocked}
    gamepad={gamepadActive}
    {toggle}
    {lock}
    paste={() => {
      void paste();
    }}
    {keyboard}
  >
    {#snippet trailing()}{@render draftToggle()}{@render quickMenu(
        "top",
        "icon-sm",
        "ghost",
      )}{/snippet}
  </SoftKeys>
</div>
