<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import { uploadFile } from "../../src/api";
  import ChatView from "../../src/components/chat/ChatView.svelte";
  import { chatActions } from "../../src/chat-api";
  import { createChatSocket } from "../../src/hooks/chat-socket.svelte";
  import { useChatUpload } from "../../src/hooks/upload.svelte";
  import type { ChatCodeLink } from "../../src/markdown-link";
  import {
    createTerminalDrafts,
    type TerminalDrafts,
  } from "../../src/terminal-draft.svelte";

  // The chat view on its real channel and commands, as the session page
  // wires it, without the terminal around it.
  let { id, drafts: providedDrafts }: { id: string; drafts?: TerminalDrafts } =
    $props();
  const session = untrack(() => id);
  const chat = createChatSocket(session);
  chat.open();
  onDestroy(() => chat.dispose());
  const drafts = untrack(() => providedDrafts ?? createTerminalDrafts());
  const draft = drafts.get(session);
  const actions = chatActions(session);
  let reviewed = $state("");
  let openedLocation = $state<ChatCodeLink | null>(null);
  let error = $state("");
  let terminalRequests = $state(0);
  const uploads = useChatUpload(
    (file, progress, signal) => uploadFile(session, file, progress, signal),
    (message) => {
      error = message;
    },
    {
      get: () => draft.attachments,
      set: (next) => drafts.setAttachments(draft, next),
    },
  );
</script>

<div style="height: 800px">
  <ChatView
    model={chat.model}
    chat={chat.state}
    connection={chat.connection}
    {draft}
    {drafts}
    {uploads}
    {actions}
    onreview={(path) => (reviewed = path)}
    onopenlocation={(target) => (openedLocation = target)}
    onterminal={() => (terminalRequests += 1)}
  />
</div>
<output aria-label="Reviewed path">{reviewed}</output>
<output aria-label="Opened location"
  >{openedLocation ? JSON.stringify(openedLocation) : ""}</output
>
<output aria-label="Draft">{draft.text}</output>
<output aria-label="Upload error">{error}</output>
<output aria-label="Terminal requests">{terminalRequests}</output>
