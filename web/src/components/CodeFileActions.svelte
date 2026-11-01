<script lang="ts" module>
  import type { CodeNode } from "../schemas";
  /** A tree entry the menu acts on; the root has the empty path. */
  export type FileTarget = Pick<CodeNode, "nodeId" | "path" | "name" | "kind">;
</script>

<script lang="ts">
  import { errorMessage } from "../error";
  import { untrack, type Component } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import type { z } from "zod";
  import { api, apiBlob, uploadForm } from "../api";
  import { saveBlob } from "../download";
  import { formatBytes, rawError } from "../code-image";
  import {
    CodeContextSchema,
    CodeCopyRequestSchema,
    CodeMkdirRequestSchema,
    CodeNameSchema,
    CodeMutationSchema,
    CodeRenameRequestSchema,
    CodeUsageSchema,
    NoContentSchema,
    type CodeNavigatorState,
  } from "../schemas";
  import type { UploadProgress } from "../hooks/upload.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import * as Drawer from "$lib/components/ui/drawer";
  import { ListButton } from "$lib/components/ui/list-button";
  import { Separator } from "$lib/components/ui/separator";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import FileUploadProgress from "./FileUploadProgress.svelte";
  import ResponsiveDialog from "./ResponsiveDialog.svelte";
  import UploadFileInput from "./UploadFileInput.svelte";
  import IconClipboard from "~icons/tabler/clipboard";
  import IconCopy from "~icons/tabler/copy";
  import IconDownload from "~icons/tabler/download";
  import IconEye from "~icons/tabler/eye";
  import IconFileZip from "~icons/tabler/file-zip";
  import IconFolderPlus from "~icons/tabler/folder-plus";
  import IconPencil from "~icons/tabler/pencil";
  import IconScissors from "~icons/tabler/scissors";
  import IconTrash from "~icons/tabler/trash";
  import IconUpload from "~icons/tabler/upload";

  type Action = {
    label: string;
    icon?: Component;
    run: () => void;
    disabled?: boolean;
    destructive?: boolean;
    // Actions of one group sit between separators.
    group: number;
  };
  type Clipboard = NonNullable<CodeNavigatorState["clipboard"]>;

  // The explorer's context menu: file operations on one tree entry, with the
  // copy/cut clipboard, dialogs and transfer progress they need.
  let {
    id,
    context,
    includeIgnored,
    openFile,
    changed,
    restore,
  }: {
    id: string;
    context: z.infer<typeof CodeContextSchema> | undefined;
    includeIgnored: boolean;
    openFile: (path: string, nodeId: string) => void;
    /** After a change; removed lists paths that no longer exist. */
    changed: (removed: string[]) => void;
    /** The copy/cut clipboard to start with. */
    restore?: Clipboard;
  } = $props();

  const base = $derived(`/sessions/${id}/code`);
  const writable = $derived(!!context?.writableAvailable);
  let menuOpen = $state(false);
  let target = $state<FileTarget>();
  let anchor = $state<HTMLElement | { getBoundingClientRect(): DOMRect }>();
  let returnFocus: HTMLElement | undefined;
  let clipboard = $state<Clipboard | undefined>(untrack(() => restore));
  let notice = $state("");
  let error = $state("");
  let naming = $state<{ kind: "rename" | "mkdir"; node: FileTarget }>();
  let name = $state("");
  let busy = $state(false);
  let dialogError = $state("");
  let removing = $state<{ node: FileTarget; detail: string }>();
  let confirmOpen = $state(false);
  let picker = $state<UploadFileInput>();
  let pickerTarget: FileTarget | undefined;
  let progress = $state<UploadProgress | null>(null);
  let uploadController: AbortController | undefined;
  let download = $state<{ name: string; bytes: number }>();
  let downloadController: AbortController | undefined;
  const pasteReady = $derived(
    !!clipboard && clipboard.contextId === context?.contextId,
  );
  // Narrow windows get a bottom action sheet with full touch targets, at the
  // breakpoint where ResponsiveDialog turns into a drawer.
  const desktop = new MediaQuery("(min-width: 640px)");
  let nameInput = $state<HTMLInputElement | null>(null);
  const rootTarget = $derived<FileTarget>({
    nodeId: "",
    path: "",
    name: context?.rootPath.split("/").filter(Boolean).at(-1) ?? "根目录",
    kind: "directory",
  });
  const actions = $derived.by((): Action[] => {
    const node = target;
    if (!node) return [];
    if (node.kind === "boundary" || node.kind === "special")
      return [
        { label: "此类条目不支持文件操作", disabled: true, group: 0, run() {} },
      ];
    const folder = node.kind === "directory";
    const editable = node.path !== "" && (node.kind === "file" || folder);
    const hold = (mode: Clipboard["mode"]) => () => {
      clipboard = { contextId: context?.contextId ?? "", nodes: [node], mode };
    };
    const list: Action[] = [];
    if (node.kind === "file")
      list.push(
        {
          label: "打开",
          icon: IconEye,
          group: 0,
          run: () => openFile(node.path, node.nodeId),
        },
        {
          label: "下载",
          icon: IconDownload,
          group: 0,
          run: () => void save(node, false),
        },
      );
    if (folder)
      list.push(
        {
          label: "下载为 ZIP",
          icon: IconFileZip,
          group: 0,
          run: () => void save(node, true),
        },
        {
          label: "上传到此目录",
          icon: IconUpload,
          group: 1,
          disabled: !writable,
          run: () => {
            pickerTarget = node;
            picker?.open();
          },
        },
        {
          label: "新建文件夹",
          icon: IconFolderPlus,
          group: 1,
          disabled: !writable,
          run: () => startNaming("mkdir", node),
        },
      );
    if (editable)
      list.push(
        {
          label: "重命名",
          icon: IconPencil,
          group: 2,
          disabled: !writable,
          run: () => startNaming("rename", node),
        },
        { label: "复制", icon: IconCopy, group: 2, run: hold("copy") },
        {
          label: "剪切",
          icon: IconScissors,
          group: 2,
          disabled: !writable,
          run: hold("move"),
        },
      );
    if (folder)
      list.push({
        label: `粘贴${pasteReady && clipboard ? `（${clipboard.mode === "move" ? "移动" : "复制"} ${clipboard.nodes[0]?.name ?? ""}）` : ""}`,
        icon: IconClipboard,
        group: 2,
        disabled: !writable || !pasteReady,
        run: () => void paste(node),
      });
    if (node.path !== "")
      list.push({
        label: "删除…",
        icon: IconTrash,
        group: 3,
        destructive: true,
        disabled: !writable,
        run: () => void askRemove(node),
      });
    return list;
  });
  // Renaming preselects the name without its extension, so typing replaces
  // only the stem; the field keeps focus as the user edits.
  $effect(() => {
    const input = nameInput;
    const current = naming;
    if (!input || !current) return;
    untrack(() => {
      const dot =
        current.kind === "rename" && current.node.kind === "file"
          ? name.lastIndexOf(".")
          : -1;
      input.focus();
      input.setSelectionRange(0, dot > 0 ? dot : name.length);
    });
  });

  /**
   * Opens the menu for node, anchored at the control that asked or at the
   * pointer position of a context-menu event; focus then returns to row.
   */
  export function show(
    node: FileTarget,
    at: HTMLElement | { x: number; y: number },
    row: HTMLElement,
  ) {
    target = node;
    returnFocus = row;
    anchor =
      at instanceof HTMLElement
        ? at
        : { getBoundingClientRect: () => new DOMRect(at.x, at.y, 0, 0) };
    menuOpen = true;
  }

  /** The pending copy/cut, to carry into another instance. */
  export function capture(): Clipboard | undefined {
    if (!clipboard) return undefined;
    const { contextId, nodes, mode } = clipboard;
    return {
      contextId,
      mode,
      nodes: nodes.map(({ nodeId, path, name, kind }) => ({
        nodeId,
        path,
        name,
        kind,
      })),
    };
  }

  /** Why files cannot be uploaded right now; empty when they can. */
  export function uploadRefusal(): string {
    if (!writable) return "当前目录只读，无法上传";
    return uploadController ? "请等待当前上传完成" : "";
  }

  /** Reports a refused operation in the strip above the tree. */
  export function refuse(message: string) {
    notice = "";
    error = message;
  }

  function query(node: FileTarget, extra: Record<string, string> = {}) {
    return new URLSearchParams({
      contextId: context?.contextId ?? "",
      nodeId: node.nodeId,
      ...extra,
    });
  }

  // Runs one operation, reporting its outcome in the strip above the tree.
  // A refusal reported meanwhile gives way to the outcome.
  async function run(action: () => Promise<string | void>) {
    error = "";
    notice = "";
    try {
      const outcome = (await action()) ?? "";
      error = "";
      notice = outcome;
    } catch (e) {
      notice = "";
      error = errorMessage(e);
    }
  }

  // Drops clipboard entries at or below paths that no longer exist.
  function forget(paths: string[]) {
    if (!clipboard) return;
    const nodes = clipboard.nodes.filter(
      (n) => !paths.some((p) => n.path === p || n.path.startsWith(p + "/")),
    );
    clipboard = nodes.length ? { ...clipboard, nodes } : undefined;
  }

  function startNaming(kind: "rename" | "mkdir", node: FileTarget) {
    naming = { kind, node };
    name = kind === "rename" ? node.name : "";
    dialogError = "";
  }

  async function submitName() {
    if (!naming || !context) return;
    const { kind, node } = naming;
    const checked = CodeNameSchema.safeParse(name);
    if (!checked.success) {
      dialogError = checked.error.issues[0]?.message ?? "名称无效";
      return;
    }
    busy = true;
    dialogError = "";
    try {
      const result =
        kind === "rename"
          ? await api(
              `${base}/rename`,
              CodeMutationSchema,
              "POST",
              { contextId: context.contextId, nodeId: node.nodeId, name },
              undefined,
              CodeRenameRequestSchema,
            )
          : await api(
              `${base}/mkdir`,
              CodeMutationSchema,
              "POST",
              { contextId: context.contextId, parentNodeId: node.nodeId, name },
              undefined,
              CodeMkdirRequestSchema,
            );
      naming = undefined;
      error = "";
      const placed = result.nodes[0]?.name ?? name;
      notice = placed === name ? "" : `名称已被占用，已改为 ${placed}`;
      if (kind === "rename" && placed !== node.name) {
        forget([node.path]);
        changed([node.path]);
      } else changed([]);
    } catch (e) {
      dialogError = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  async function paste(into: FileTarget) {
    const current = clipboard;
    if (!current || !context) return;
    await run(async () => {
      const result = await api(
        `${base}/copy`,
        CodeMutationSchema,
        "POST",
        {
          contextId: context.contextId,
          nodeIds: current.nodes.map((n) => n.nodeId),
          targetNodeId: into.nodeId,
          mode: current.mode,
        },
        undefined,
        CodeCopyRequestSchema,
      );
      if (current.mode === "move") {
        clipboard = undefined;
        changed(
          current.nodes
            .filter((n, i) => result.nodes[i]?.path !== n.path)
            .map((n) => n.path),
        );
      } else changed([]);
      const renamed = result.nodes.filter(
        (n, i) => n.name !== current.nodes[i]?.name,
      );
      return renamed.length
        ? `同名条目已存在，已粘贴为 ${renamed.map((n) => n.name).join("、")}`
        : `已${current.mode === "move" ? "移动" : "复制"} ${result.nodes.length} 项`;
    });
  }

  async function askRemove(node: FileTarget) {
    await run(async () => {
      let detail = `将删除${node.kind === "symlink" ? "符号链接（不影响其目标）" : "文件"} ${node.path}。此操作不能撤销。`;
      if (node.kind === "directory") {
        const usage = await api(
          `${base}/usage?${query(node)}`,
          CodeUsageSchema,
        );
        detail = `将递归删除文件夹 ${node.path} 及其中 ${usage.entries - 1}${usage.limited ? "+" : ""} 个条目（${formatBytes(usage.bytes)}）。此操作不能撤销。`;
      }
      removing = { node, detail };
      confirmOpen = true;
    });
  }

  async function remove() {
    const node = removing?.node;
    confirmOpen = false;
    if (!node) return;
    await run(async () => {
      await api(
        `${base}/node?${query(node, node.kind === "directory" ? { recursive: "1" } : {})}`,
        NoContentSchema,
        "DELETE",
      );
      forget([node.path]);
      changed([node.path]);
      return `已删除 ${node.name}`;
    });
  }

  /**
   * Uploads files into a folder one by one, replacing any running upload;
   * resolves to the outcome shown in the strip.
   */
  export async function upload(files: File[], into: FileTarget) {
    if (!files.length) return "";
    uploadController?.abort();
    const controller = new AbortController();
    uploadController = controller;
    const stored: string[] = [];
    await run(async () => {
      try {
        for (const [index, file] of files.entries()) {
          const current = {
            name: file.name,
            percent: 0,
            index: index + 1,
            total: files.length,
            waiting: false,
          };
          progress = current;
          const result = await uploadForm(
            `${base}/upload?${query(into)}`,
            file,
            CodeMutationSchema,
            (percent) => (progress = { ...current, percent }),
            controller.signal,
          );
          stored.push(...result.nodes.map((n) => n.name));
        }
        return `已上传 ${stored.join("、")}`;
      } finally {
        if (uploadController === controller) {
          uploadController = undefined;
          progress = null;
        }
        if (stored.length) changed([]);
      }
    });
    return error || notice;
  }

  async function save(node: FileTarget, archive: boolean) {
    downloadController?.abort();
    const controller = new AbortController();
    downloadController = controller;
    const filename = archive ? `${node.name}.zip` : node.name;
    download = { name: filename, bytes: 0 };
    error = "";
    notice = "";
    try {
      const blob = await apiBlob(
        archive
          ? `${base}/archive?${query(node, { includeIgnored: String(includeIgnored) })}`
          : `${base}/node?${query(node)}`,
        controller.signal,
        (bytes) => {
          if (download) download.bytes = bytes;
        },
      );
      if (!controller.signal.aborted)
        saveBlob(returnFocus?.ownerDocument ?? document, blob, filename);
    } catch (e) {
      if (!controller.signal.aborted) error = rawError(e, "下载");
    } finally {
      if (downloadController === controller) {
        downloadController = undefined;
        download = undefined;
      }
    }
  }

  $effect(() => () => {
    uploadController?.abort();
    downloadController?.abort();
  });
</script>

<UploadFileInput
  bind:this={picker}
  upload={async (files) => {
    if (pickerTarget) await upload(files, pickerTarget);
  }}
  label="选择上传到此目录的文件"
/>

{#if progress}<FileUploadProgress
    variant="card"
    {progress}
    cancel={() => uploadController?.abort()}
  />{/if}
{#if download}<div
    class="flex shrink-0 items-center gap-2 px-2 py-1 text-xs"
    role="status"
  >
    <span class="min-w-0 flex-1 truncate"
      >正在{download.name.endsWith(".zip") ? "压缩并" : ""}下载 {download.name}
      · {formatBytes(download.bytes)}</span
    >
    <Button
      variant="ghost"
      size="xs"
      onclick={() => downloadController?.abort()}>取消</Button
    >
  </div>{/if}
{#if error}<p
    role="alert"
    class="shrink-0 px-2 py-1 text-xs break-all text-destructive"
  >
    {error}
  </p>{:else if notice}<p
    role="status"
    class="shrink-0 px-2 py-1 text-xs break-all text-muted-foreground"
  >
    {notice}
  </p>{/if}

{#if pasteReady && clipboard}<div
    class="flex shrink-0 items-center gap-1 border-y bg-muted/50 px-2 py-1 text-xs"
    role="status"
    aria-label="剪贴板"
  >
    <span class="min-w-0 flex-1 truncate" title="在目标文件夹的菜单中选择“粘贴”"
      >{clipboard.mode === "move" ? "已剪切" : "已复制"}
      {clipboard.nodes.map((n) => n.name).join("、")}</span
    >
    <Button
      variant="ghost"
      size="xs"
      disabled={!writable}
      onclick={() => void paste(rootTarget)}>粘贴到根目录</Button
    >
    <Button variant="ghost" size="xs" onclick={() => (clipboard = undefined)}
      >清除</Button
    >
  </div>{/if}

{#if desktop.current}
  <DropdownMenu.Root bind:open={menuOpen}>
    <DropdownMenu.Content
      customAnchor={anchor}
      align="start"
      class="w-56"
      aria-label={target ? `文件操作 ${target.name}` : "文件操作"}
      onCloseAutoFocus={(event) => {
        event.preventDefault();
        returnFocus?.focus();
      }}
    >
      {#each actions as action, index (action.label)}
        {#if index > 0 && action.group !== actions[index - 1]?.group}<DropdownMenu.Separator
          />{/if}
        <DropdownMenu.Item
          variant={action.destructive ? "destructive" : "default"}
          disabled={action.disabled}
          onSelect={action.run}
          >{#if action.icon}<action.icon aria-hidden="true" />{/if}<span
            class="truncate">{action.label}</span
          ></DropdownMenu.Item
        >
      {/each}
    </DropdownMenu.Content>
  </DropdownMenu.Root>
{:else}
  <Drawer.Root bind:open={menuOpen}>
    <Drawer.Content
      aria-label={target ? `文件操作 ${target.name}` : "文件操作"}
    >
      <Drawer.Header>
        <Drawer.Title>{target?.name ?? ""}</Drawer.Title>
        <Drawer.Description
          >{target?.path
            ? `${context?.rootPath ?? ""}/${target.path}`
            : (context?.rootPath ?? "")}</Drawer.Description
        >
      </Drawer.Header>
      <div class="flex min-h-0 flex-col overflow-y-auto px-2 pb-safe-4">
        {#each actions as action, index (action.label)}
          {#if index > 0 && action.group !== actions[index - 1]?.group}<div
              class="px-3 py-1"
            >
              <Separator />
            </div>{/if}
          <ListButton
            variant="action-sheet"
            data-destructive={action.destructive}
            disabled={action.disabled}
            onclick={() => {
              // Run inside the tap: the upload picker needs the gesture.
              menuOpen = false;
              action.run();
            }}
            >{#if action.icon}<action.icon aria-hidden="true" />{/if}<span
              class="min-w-0 truncate">{action.label}</span
            ></ListButton
          >
        {/each}
        <Button
          variant="outline"
          class="mt-2 w-full"
          onclick={() => (menuOpen = false)}>取消</Button
        >
      </div>
    </Drawer.Content>
  </Drawer.Root>
{/if}

{#if naming}
  {@const { kind, node } = naming}
  <ResponsiveDialog
    title={kind === "rename" ? "重命名" : "新建文件夹"}
    description={kind === "rename"
      ? `在原目录中重命名 ${node.path}；名称已被占用时自动追加 (n)。`
      : `在 ${node.path || "根目录"} 中新建文件夹；名称已被占用时自动追加 (n)。`}
    onclose={() => (naming = undefined)}
    {busy}
  >
    <form
      class="space-y-4"
      onsubmit={(event) => {
        event.preventDefault();
        void submitName();
      }}
    >
      <div class="space-y-2">
        <Label for="code-file-name">名称</Label>
        <Input
          id="code-file-name"
          bind:ref={nameInput}
          bind:value={name}
          required
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
        />
      </div>
      {#if dialogError}<p
          role="alert"
          class="text-sm break-all text-destructive"
        >
          {dialogError}
        </p>{/if}
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          disabled={busy}
          onclick={() => (naming = undefined)}>取消</Button
        >
        <Button type="submit" disabled={busy || !name}
          >{kind === "rename" ? "重命名" : "新建"}</Button
        >
      </div>
    </form>
  </ResponsiveDialog>
{/if}

<ConfirmDialog
  bind:open={confirmOpen}
  title={`删除 ${removing?.node.name ?? ""}`}
  onconfirm={() => void remove()}
>
  {removing?.detail ?? ""}
</ConfirmDialog>
