import { getContext, setContext } from "svelte";
import { z } from "zod";
import { navigate, store, stored } from "./api";
import type { Archived } from "./hooks/archived.svelte";
import type { LaunchConfigs } from "./hooks/launch-configs.svelte";
import type { Repositories } from "./hooks/repositories.svelte";
import type { ServerSetup } from "./hooks/server-setup.svelte";
import type { SessionList } from "./hooks/session-list.svelte";
import type { LaunchEntry } from "./newSessionDefaults";
import type { SessionInfo } from "./schemas";
import type { TerminalDrafts } from "./terminal-draft.svelte";

// The app shell around the home page, quick start and session pages: which
// mode the sidebar shows, and whether it is collapsed (desktop) or open as a
// drawer (phones). The mode and the collapse survive reloads on this device.

export type ShellMode = "chat" | "workbench";
const ModeSchema = z.enum(["chat", "workbench"]);
const CollapsedSchema = z.enum(["true", "false"]);

// First use, or an invalid stored value, starts in the workbench.
let mode = $state<ShellMode>(
  ModeSchema.safeParse(stored("shellMode")).data ?? "workbench",
);
let collapsed = $state(
  CollapsedSchema.safeParse(stored("sidebarCollapsed")).data === "true",
);
let drawerOpen = $state(false);

export const shellUi = {
  get mode(): ShellMode {
    return mode;
  },
  /** Remembers the mode; the sidebar follows, the page stays. */
  setMode(next: ShellMode) {
    mode = next;
    store("shellMode", next);
  },
  get collapsed() {
    return collapsed;
  },
  setCollapsed(next: boolean) {
    collapsed = next;
    store("sidebarCollapsed", String(next));
  },
  get drawerOpen() {
    return drawerOpen;
  },
  set drawerOpen(next: boolean) {
    drawerOpen = next;
  },
  /** The current mode's home page. */
  get home() {
    return mode === "chat" ? "/new" : "/";
  },
};

/**
 * Opening `/` from outside the shell (a fresh load, the settings page)
 * returns to the remembered mode: chat mode replaces the entry with `/new`,
 * so going back never lands on the workbench home first.
 */
export function enterHome() {
  if (location.pathname === "/" && mode === "chat")
    history.replaceState(
      history.state,
      "",
      "/new" + location.search + location.hash,
    );
}

/** The shell's shared data and the actions its sidebar and pages offer. */
export interface Shell {
  readonly drafts: TerminalDrafts;
  readonly sessions: SessionList;
  readonly repositories: Repositories;
  readonly archived: Archived;
  readonly setup: ServerSetup;
  readonly launchConfigs: LaunchConfigs;
  /** Opens the new-session dialog, optionally at a contextual entry. */
  start(entry?: LaunchEntry): void;
  /** Opens the shared rename dialog for a live session. */
  rename(session: SessionInfo): void;
  /** Opens the port-forwarding dialog. */
  forward(): void;
  /** Opens the add-repository dialog. */
  addRepository(): void;
}

const key = Symbol("shell");
export function setShell(shell: Shell) {
  setContext(key, shell);
}
/** The shell, or undefined for a page rendered outside it (pop-outs, tests). */
export function findShell(): Shell | undefined {
  return getContext<Shell | undefined>(key);
}
export function getShell(): Shell {
  const shell = getContext<Shell | undefined>(key);
  if (!shell) throw new Error("页面必须位于应用外壳中");
  return shell;
}

/**
 * Opens a session from the sidebar or a home card. In the workbench, a
 * hybrid session on its chat side takes the whole shell to chat mode;
 * anything else keeps the mode. Decided once, on the click.
 */
export function openSession(session: Pick<SessionInfo, "id" | "hybrid">) {
  if (
    mode === "workbench" &&
    session.hybrid?.state === "active" &&
    session.hybrid.side === "acp"
  )
    shellUi.setMode("chat");
  drawerOpen = false;
  navigate(`/s/${session.id}`);
}

/** Opens a page of the shell (a home page or a scope) and closes the drawer. */
export function openPage(path: string) {
  drawerOpen = false;
  if (location.pathname + location.hash === path) return;
  navigate(path);
}
