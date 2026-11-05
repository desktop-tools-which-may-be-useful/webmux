<script module lang="ts">
  import DOMPurify, { type Config } from "dompurify";
  import {
    isExternalLink,
    isWorkspaceReference,
    parseChatCodeLink,
    type ChatCodeLink,
  } from "../markdown-link";

  const forbiddenTags = [
    "image",
    "svg",
    "math",
    "use",
    "foreignobject",
    "picture",
    "video",
    "audio",
    "source",
    "track",
    "iframe",
    "object",
    "embed",
    "form",
    "input",
    "button",
    "textarea",
    "select",
    "option",
    "link",
    "meta",
    "base",
    "style",
    // Image maps would keep a raw relative `href` on `<area>`.
    "map",
    "area",
  ];
  // Authored `id` / `name` could clobber application elements.
  const forbiddenAttributes = ["style", "srcset", "background", "id", "name"];

  // Each variant owns its DOMPurify instance: hooks are instance-wide state,
  // so the file preview's link and image handling never reaches chat messages.
  let allowMessageCodeLinks = false;
  const pendingCodeHrefs = new WeakMap<Element, string>();
  const messagePurifier = DOMPurify(window);
  messagePurifier.addHook("uponSanitizeAttribute", (node, data) => {
    if (data.attrName.startsWith("data-md-")) {
      data.keepAttr = false;
      return;
    }
    // Capture valid code-location targets before DOMPurify's built-in URI
    // filter strips non-HTTP schemes (`file:///...`) or colon line suffixes
    // (`Makefile:15`), and never leave a non-external `href` on `<a>`.
    if (
      node.tagName === "A" &&
      data.attrName === "href" &&
      !isExternalLink(data.attrValue)
    ) {
      if (allowMessageCodeLinks && parseChatCodeLink(data.attrValue))
        pendingCodeHrefs.set(node, data.attrValue);
      data.keepAttr = false;
    }
  });
  messagePurifier.addHook("afterSanitizeAttributes", (node) => {
    if (node.nodeType !== Node.ELEMENT_NODE) return;
    const element = node as Element;
    if (element.tagName !== "A") return;
    const href = element.getAttribute("href");
    if (href !== null && isExternalLink(href)) {
      element.setAttribute("target", "_blank");
      element.setAttribute("rel", "noopener noreferrer");
      return;
    }
    element.removeAttribute("href");
    const codeHref = pendingCodeHrefs.get(element);
    if (!codeHref) return;
    pendingCodeHrefs.delete(element);
    element.setAttribute("data-md-href", codeHref);
    element.setAttribute("role", "link");
    element.setAttribute("tabindex", "0");
  });
  const messageConfig: Config = {
    FORBID_TAGS: ["img", ...forbiddenTags],
    FORBID_ATTR: forbiddenAttributes,
    ADD_ATTR: ["target", "rel"],
  };
  function sanitizeMessageHtml(html: string, codeLinks: boolean) {
    allowMessageCodeLinks = codeLinks;
    try {
      return messagePurifier.sanitize(html, messageConfig);
    } finally {
      allowMessageCodeLinks = false;
    }
  }

  const documentPurifier = DOMPurify(window);
  // `data-md-*` attributes carry the preview's own resolved references; an
  // author must not be able to forge them.
  documentPurifier.addHook("uponSanitizeAttribute", (_node, data) => {
    if (data.attrName.startsWith("data-md-")) data.keepAttr = false;
  });
  // Workspace addresses leave `href` / `src` while the markup is still inert,
  // so the browser never requests them as application routes.
  documentPurifier.addHook("afterSanitizeAttributes", (node) => {
    if (node.nodeType !== Node.ELEMENT_NODE) return;
    const element = node as Element;
    if (element.tagName === "A") {
      const href = element.getAttribute("href");
      if (href === null) return;
      if (isExternalLink(href)) {
        element.setAttribute("target", "_blank");
        element.setAttribute("rel", "noopener noreferrer");
        return;
      }
      element.removeAttribute("href");
      if (!isWorkspaceReference(href)) return;
      element.setAttribute("data-md-href", href);
      element.setAttribute("role", "link");
      element.setAttribute("tabindex", "0");
    } else if (element.tagName === "IMG") {
      const src = element.getAttribute("src") ?? "";
      element.removeAttribute("src");
      element.setAttribute(
        isWorkspaceReference(src) ? "data-md-src" : "data-md-external",
        src,
      );
    }
  });
  const documentConfig = {
    FORBID_TAGS: forbiddenTags,
    FORBID_ATTR: forbiddenAttributes,
    ADD_ATTR: ["target", "rel"],
    RETURN_DOM_FRAGMENT: true,
  } as const satisfies Config;

  export type MarkdownHeading = { level: number; text: string; slug: string };
</script>

<script lang="ts">
  import { marked, type Token, type TokensList } from "marked";
  import { untrack } from "svelte";
  import type { CodeMarkdownState } from "../schemas";
  import IconCheck from "~icons/tabler/check";
  import IconCopy from "~icons/tabler/copy";
  import { Button } from "$lib/components/ui/button";
  import InlineNotice from "./InlineNotice.svelte";
  import { copyText } from "../clipboard";
  import { loadFenceLanguage } from "../code-language";
  import { highlightInto } from "../code-highlight";
  import {
    resolveMarkdownReference,
    targetFailure,
    type MarkdownImage,
    type MarkdownWorkspace,
  } from "../markdown-link";
  import { createSlugger, headingIdPrefix } from "../markdown-slug";

  // `document` previews a Markdown file in the code panel; `message` renders
  // an agent's chat message, with each fenced code block copyable on its own.
  let {
    text,
    variant = "document",
    path = "",
    active = true,
    workspace,
    headings = $bindable([]),
    restore,
    onopenlink,
  }: {
    text: string;
    variant?: "document" | "message";
    /** Root-relative path of the previewed file; relative links resolve against it. */
    path?: string;
    /** Inactive previews cancel image reads and release their object URLs. */
    active?: boolean;
    workspace?: MarkdownWorkspace;
    headings?: MarkdownHeading[];
    /** A captured reading position, applied after the first render with layout. */
    restore?: CodeMarkdownState;
    /** Opens a code-location link from a chat message in the code browser. */
    onopenlink?: (target: ChatCodeLink) => void;
  } = $props();

  const escapeHtml = (value: string) =>
    value.replace(
      /[&<>"']/g,
      (char) =>
        ({
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#39;",
        })[char]!,
    );
  const imageText = (label: string, address: string) =>
    `图片：${label || address}${label && address ? `（${address}）` : ""}`;

  const messageRenderer = new marked.Renderer();
  // Chat messages never load images: keep the reference as text.
  messageRenderer.image = ({ href, text }) =>
    `<span class="markdown-image">${escapeHtml(imageText(text, href))}</span>`;

  type Block = { kind: "html"; html: string } | { kind: "code"; code: string };
  // Prose between fenced code blocks is rendered (and sanitized) as before;
  // the code blocks themselves stay plain text next to their copy buttons.
  function blocks(source: string, codeLinks: boolean): Block[] {
    const tokens = marked.lexer(source, { gfm: true });
    const out: Block[] = [];
    let run: Token[] = [];
    const flush = () => {
      if (!run.length) return;
      const list = Object.assign(run, { links: tokens.links }) as TokensList;
      out.push({
        kind: "html",
        html: sanitizeMessageHtml(
          marked.parser(list, { gfm: true, renderer: messageRenderer }),
          codeLinks,
        ),
      });
      run = [];
    };
    for (const token of tokens) {
      if (token.type !== "code") {
        run.push(token);
        continue;
      }
      flush();
      out.push({ kind: "code", code: token.text });
    }
    flush();
    return out;
  }

  const parts = $derived(
    variant === "message" ? blocks(text, Boolean(onopenlink)) : [],
  );
  let copied = $state(-1);
  async function copy(index: number, code: string) {
    if (await copyText(code)) copied = index;
  }

  // A relative image waiting in its placeholder until it is near the viewport.
  type Picture = {
    element: HTMLElement;
    address: string;
    alt: string;
    path: string;
    width: string;
    height: string;
  };
  const maxImages = 50;
  const parallelImages = 2;
  let messageRoot = $state<HTMLDivElement>();
  let region = $state<HTMLDivElement>();
  let content = $state<HTMLDivElement>();
  let pictures = $state.raw<Picture[]>([]);
  let notice = $state("");
  let retryable = $state(false);
  let retry: (() => void) | undefined;
  let rendering = 0;

  function placeholder(image: Element) {
    const span = image.ownerDocument.createElement("span");
    span.className = "markdown-image";
    const alt = image.getAttribute("alt") ?? "";
    const address =
      image.getAttribute("data-md-src") ??
      image.getAttribute("data-md-external") ??
      "";
    span.textContent = imageText(alt, address);
    image.replaceWith(span);
    return { span, alt, address };
  }

  // Renders, sanitizes and enhances the document: image placeholders,
  // heading anchors and (asynchronously) highlighted code blocks.
  $effect(() => {
    const element = content;
    if (variant !== "document" || !element) return;
    const source = text;
    const base = path;
    const fragment = documentPurifier.sanitize(
      marked.parse(source, { async: false, gfm: true }),
      documentConfig,
    );
    const found: Picture[] = [];
    for (const image of fragment.querySelectorAll("img")) {
      const relative = image.getAttribute("data-md-src");
      const { span, alt, address } = placeholder(image);
      if (relative === null) continue;
      const reference = resolveMarkdownReference(base, relative);
      if (reference.kind !== "path") {
        span.textContent = `${reference.kind === "outside" ? "图片不在工作区内" : "图片地址无效"}（${address}）`;
        continue;
      }
      found.push({
        element: span,
        address,
        alt,
        path: reference.path,
        width: image.getAttribute("width") ?? "",
        height: image.getAttribute("height") ?? "",
      });
    }
    const slug = createSlugger();
    const outline: MarkdownHeading[] = [];
    for (const heading of fragment.querySelectorAll("h1, h2, h3, h4, h5, h6")) {
      const label = heading.textContent ?? "";
      const id = slug(label);
      heading.id = headingIdPrefix + id;
      outline.push({
        level: Number(heading.tagName[1]),
        text: label,
        slug: id,
      });
    }
    element.replaceChildren(fragment);
    const generation = ++rendering;
    for (const code of element.querySelectorAll("pre > code")) {
      const language = /(?:^|\s)language-(\S+)/.exec(code.className)?.[1];
      if (!language) continue;
      const plain = code.textContent ?? "";
      void loadFenceLanguage(language)
        .then((support) => {
          if (support && generation === rendering)
            highlightInto(code, plain, support);
        })
        // A grammar chunk that fails to load leaves the block as plain text.
        .catch(() => undefined);
    }
    untrack(() => {
      headings = outline;
      pictures = found;
      notice = "";
      applyRestore();
    });
  });

  // Until applied, `capture()` returns it unchanged.
  let pending = untrack(() => restore);
  // The position last measured while the preview had layout; a hidden tab
  // reports this one.
  let known: CodeMarkdownState = { heading: "", offset: 0 };
  // A restored position is held while late images change the layout above
  // it, until the reader moves on.
  let anchor: CodeMarkdownState | undefined;
  function measure(): CodeMarkdownState {
    if (!region || !content || region.clientHeight === 0) return known;
    const top = region.getBoundingClientRect().top;
    let last: Element | undefined;
    for (const heading of content.querySelectorAll("h1, h2, h3, h4, h5, h6")) {
      if (heading.getBoundingClientRect().top > top + 1) break;
      last = heading;
    }
    const range = region.scrollHeight - region.clientHeight;
    known = last
      ? {
          heading: last.id.slice(headingIdPrefix.length),
          offset: top - last.getBoundingClientRect().top,
        }
      : {
          heading: "",
          offset: range > 0 ? Math.min(1, region.scrollTop / range) : 0,
        };
    return known;
  }
  export function capture(): CodeMarkdownState {
    return pending ?? measure();
  }
  function place(state: CodeMarkdownState) {
    if (!region || !content) return;
    if (!state.heading) {
      region.scrollTop =
        state.offset * (region.scrollHeight - region.clientHeight);
      return;
    }
    const heading = content.querySelector(
      `#${CSS.escape(headingIdPrefix + state.heading)}`,
    );
    if (heading)
      region.scrollTop +=
        heading.getBoundingClientRect().top -
        region.getBoundingClientRect().top +
        state.offset;
  }
  function applyRestore() {
    const state = pending;
    if (!state || !region || region.clientHeight === 0) return;
    pending = undefined;
    anchor = state;
    known = state;
    place(state);
  }
  // Defers a restore for a hidden tab, keeps a restored position anchored
  // while images load, and tracks the position while scrolling.
  $effect(() => {
    const scroller = region;
    const inner = content;
    if (variant !== "document" || !scroller || !inner) return;
    const observer = new ResizeObserver(() => {
      if (pending) applyRestore();
      else if (anchor && scroller.clientHeight > 0) place(anchor);
    });
    observer.observe(scroller);
    observer.observe(inner);
    let measuring = 0;
    const scrolled = () => {
      if (pending || measuring) return;
      measuring = requestAnimationFrame(() => {
        measuring = 0;
        if (!pending) measure();
      });
    };
    const release = () => {
      anchor = undefined;
    };
    const inputs = ["wheel", "pointerdown", "touchstart", "keydown"] as const;
    scroller.addEventListener("scroll", scrolled, { passive: true });
    for (const type of inputs)
      scroller.addEventListener(type, release, { passive: true });
    return () => {
      observer.disconnect();
      cancelAnimationFrame(measuring);
      scroller.removeEventListener("scroll", scrolled);
      for (const type of inputs) scroller.removeEventListener(type, release);
    };
  });

  // Loads images near the viewport through the workspace, at most two at a
  // time and fifty per document. Deactivating or re-rendering cancels the
  // reads and releases every object URL.
  $effect(() => {
    const list = pictures;
    const scroller = region;
    const source = workspace;
    // Without workspace access the placeholders stay as text.
    if (!active || !scroller || !source || !list.length) return;
    const controller = new AbortController();
    const urls: string[] = [];
    const queue: Picture[] = [];
    const busy: Picture[] = [];
    let running = 0;
    const pump = () => {
      while (running < parallelImages && queue.length) {
        const picture = queue.shift()!;
        running++;
        void load(picture).finally(() => {
          running--;
          if (!controller.signal.aborted) pump();
        });
      }
    };
    const fail = (picture: Picture, reason: string) => {
      picture.element.textContent = `${reason}（${picture.address}）`;
    };
    const load = async (picture: Picture) => {
      picture.element.setAttribute("aria-busy", "true");
      let result: MarkdownImage;
      try {
        result = await source.image(picture.path, controller.signal);
      } catch (error) {
        result = { ok: false, reason: targetFailure("图片", error) };
      }
      if (controller.signal.aborted) return;
      picture.element.removeAttribute("aria-busy");
      if (!result.ok) {
        fail(picture, result.reason);
        if (result.retry) {
          busy.push(picture);
          retryable = true;
        }
        return;
      }
      const url = URL.createObjectURL(result.blob);
      urls.push(url);
      const image = picture.element.ownerDocument.createElement("img");
      image.alt = picture.alt;
      for (const name of ["width", "height"] as const)
        if (/^\d+$/.test(picture[name]))
          image.setAttribute(name, picture[name]);
      image.src = url;
      picture.element.replaceChildren(image);
      picture.element.dataset.mdOpen = result.path;
      picture.element.setAttribute("role", "link");
      picture.element.setAttribute("tabindex", "0");
      picture.element.title = `在代码面板中打开 ${result.path}`;
    };
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          observer.unobserve(entry.target);
          const picture = list.find((item) => item.element === entry.target);
          if (picture) queue.push(picture);
        }
        pump();
      },
      { root: scroller, rootMargin: "100% 0px" },
    );
    list.forEach((picture, index) => {
      if (index < maxImages) observer.observe(picture.element);
      else fail(picture, "图片过多，未加载");
    });
    retry = () => {
      retryable = false;
      queue.push(...busy.splice(0));
      pump();
    };
    return () => {
      controller.abort();
      observer.disconnect();
      for (const url of urls) URL.revokeObjectURL(url);
      retry = undefined;
      retryable = false;
      for (const picture of list) {
        picture.element.removeAttribute("aria-busy");
        picture.element.removeAttribute("data-md-open");
        picture.element.removeAttribute("role");
        picture.element.removeAttribute("tabindex");
        picture.element.removeAttribute("title");
        picture.element.textContent = imageText(picture.alt, picture.address);
      }
    };
  });

  /** Scrolls the preview to a heading slug; returns false and explains when it is missing. */
  export function scrollToHeading(slug: string) {
    const heading = content?.querySelector(
      `#${CSS.escape(headingIdPrefix + slug)}`,
    );
    if (!heading) {
      notice = `未找到标题“${slug}”`;
      return false;
    }
    notice = "";
    // An explicit navigation replaces any restored position.
    pending = anchor = undefined;
    heading.scrollIntoView({ block: "start" });
    return true;
  }

  async function follow(element: Element) {
    const opened = element.getAttribute("data-md-open");
    if (opened !== null) {
      notice = (await workspace?.open(opened)) ?? "";
      return;
    }
    const reference = resolveMarkdownReference(
      path,
      element.getAttribute("data-md-href") ?? "",
    );
    if (reference.kind === "outside") notice = "链接目标不在工作区内";
    else if (reference.kind === "invalid") notice = "链接地址无效";
    else if (reference.kind === "anchor") scrollToHeading(reference.fragment);
    else if (reference.path === path) {
      if (reference.fragment) scrollToHeading(reference.fragment);
      else region?.scrollTo({ top: 0 });
    } else if (!workspace) notice = "无法在此打开链接";
    else {
      notice = "";
      notice = await workspace.open(reference.path, reference.fragment);
    }
  }

  // Delegated so links and loaded images inside the rendered markup work the
  // same with or without modifier keys, and never navigate the app.
  $effect(() => {
    const element = variant === "message" ? messageRoot : content;
    if (!element) return;
    const hit = (event: Event) =>
      (event.target as Element | null)?.closest(
        "[data-md-href], [data-md-open]",
      );
    const activateLink = (link: Element) => {
      if (variant === "message") {
        const parsed = parseChatCodeLink(
          link.getAttribute("data-md-href") ?? "",
        );
        if (parsed) onopenlink?.(parsed);
        return;
      }
      void follow(link);
    };
    const click = (event: MouseEvent) => {
      const link = hit(event);
      if (!link) return;
      event.preventDefault();
      activateLink(link);
    };
    const keydown = (event: KeyboardEvent) => {
      const link = hit(event);
      if (!link || event.key !== "Enter") return;
      event.preventDefault();
      activateLink(link);
    };
    element.addEventListener("click", click);
    element.addEventListener("keydown", keydown);
    return () => {
      element.removeEventListener("click", click);
      element.removeEventListener("keydown", keydown);
    };
  });
</script>

{#if variant === "message"}
  <div
    bind:this={messageRoot}
    class="markdown-preview markdown-message text-sm"
  >
    {#each parts as part, i (i)}
      {#if part.kind === "html"}
        <!-- Sanitized by DOMPurify; raw HTML never reaches the page unsanitized. -->
        {@html part.html}
      {:else}
        <div class="relative">
          <pre><code>{part.code}</code></pre>
          <Button
            variant="ghost"
            size="icon-xs"
            class="absolute top-1 right-1"
            aria-label={copied === i ? "已复制代码" : "复制代码"}
            title={copied === i ? "已复制" : "复制代码"}
            onclick={() => copy(i, part.code)}
            >{#if copied === i}<IconCheck aria-hidden="true" />{:else}<IconCopy
                aria-hidden="true"
              />{/if}</Button
          >
        </div>
      {/if}
    {/each}
  </div>
{:else}
  <div class="flex h-full min-h-0 flex-col">
    {#if notice || retryable}
      <div class="flex shrink-0 flex-wrap items-center gap-2 p-2">
        {#if notice}<InlineNotice tone="warning" class="min-w-0 flex-1 py-1.5"
            >{notice}</InlineNotice
          >{/if}
        {#if retryable}<Button
            variant="outline"
            size="sm"
            onclick={() => retry?.()}>重试加载图片</Button
          >{/if}
      </div>
    {/if}
    <!-- Content is a DOMPurify fragment; raw HTML never reaches the page unsanitized. -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex (scrollable preview needs keyboard focus) -->
    <div
      bind:this={region}
      class="markdown-preview min-h-0 flex-1 overflow-auto p-4 text-sm"
      role="region"
      tabindex="0"
      aria-label="Markdown 预览"
    >
      <div bind:this={content}></div>
    </div>
  </div>
{/if}

<style>
  .markdown-preview {
    line-height: 1.6;
  }
  .markdown-preview :global(h1),
  .markdown-preview :global(h2),
  .markdown-preview :global(h3),
  .markdown-preview :global(h4),
  .markdown-preview :global(h5),
  .markdown-preview :global(h6) {
    margin: 1.2em 0 0.6em;
    font-weight: 600;
    line-height: 1.3;
  }
  .markdown-preview :global(h1) {
    font-size: 1.5em;
  }
  .markdown-preview :global(h2) {
    font-size: 1.3em;
  }
  .markdown-preview :global(h3) {
    font-size: 1.15em;
  }
  .markdown-preview > :global(:first-child),
  .markdown-preview > div > :global(:first-child) {
    margin-top: 0;
  }
  .markdown-message > :global(:last-child),
  .markdown-message > div:last-child > pre {
    margin-bottom: 0;
  }
  .markdown-preview :global(p),
  .markdown-preview :global(ul),
  .markdown-preview :global(ol),
  .markdown-preview :global(blockquote),
  .markdown-preview :global(pre),
  .markdown-preview :global(table) {
    margin: 0.8em 0;
  }
  .markdown-preview :global(ul),
  .markdown-preview :global(ol) {
    padding-left: 1.5em;
  }
  .markdown-preview :global(ul) {
    list-style: disc;
  }
  .markdown-preview :global(ol) {
    list-style: decimal;
  }
  .markdown-preview :global(li) {
    margin: 0.25em 0;
  }
  .markdown-preview :global(blockquote) {
    border-left: 3px solid var(--border);
    padding-left: 1em;
    color: var(--muted-foreground);
  }
  .markdown-preview :global(code) {
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--muted);
    padding: 0.1em 0.35em;
    font-family: var(--font-mono, monospace);
    font-size: 0.9em;
  }
  .markdown-preview :global(pre) {
    overflow-x: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--muted);
    padding: 0.75em 1em;
  }
  .markdown-preview :global(pre code) {
    border: 0;
    background: transparent;
    padding: 0;
    font-size: 0.9em;
  }
  .markdown-preview :global(a[href]),
  .markdown-preview :global(a[data-md-href]),
  .markdown-preview :global([data-md-open]) {
    border-radius: var(--radius-sm);
    color: var(--primary);
    text-decoration: underline;
    text-underline-offset: 2px;
    cursor: pointer;
  }
  .markdown-preview :global(a[href]:hover),
  .markdown-preview :global(a[data-md-href]:hover),
  .markdown-preview :global([data-md-open]:hover) {
    opacity: 0.8;
  }
  .markdown-preview :global(a[href]:focus-visible),
  .markdown-preview :global(a[data-md-href]:focus-visible),
  .markdown-preview :global([data-md-open]:focus-visible) {
    outline: 2px solid var(--ring);
    outline-offset: 2px;
  }
  .markdown-preview :global(hr) {
    margin: 1.5em 0;
    border: 0;
    border-top: 1px solid var(--border);
  }
  .markdown-preview :global(table) {
    width: 100%;
    border-collapse: collapse;
  }
  .markdown-preview :global(th),
  .markdown-preview :global(td) {
    border: 1px solid var(--border);
    padding: 0.4em 0.6em;
    text-align: left;
  }
  .markdown-preview :global(th) {
    background: var(--muted);
    font-weight: 600;
  }
  .markdown-preview :global(.markdown-image) {
    color: var(--muted-foreground);
    font-style: italic;
  }
  .markdown-preview :global(.markdown-image img) {
    display: inline-block;
    max-width: 100%;
    height: auto;
  }
</style>
