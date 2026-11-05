import { getContext, setContext } from "svelte";
import { z } from "zod";
import { navigate, store, stored } from "./api";
import type { Archived } from "./hooks/archived.svelte";
import type { LaunchConfigs } from "./hooks/launch-configs.svelte";
import type { Repositories } from "./hooks/repositories.svelte";
import type { ServerSetup } from "./hooks/server-setup.svelte";
import type { SessionList } from "./hooks/session-list.svelte";
import type { LaunchEntry } from "./newSessionDefaults";
import type { Repository, SessionInfo } from "./schemas";
import type { TerminalDrafts } from "./terminal-draft.svelte";

// The app shell around the home page, quick start and session pages: which
// mode the sidebar shows, its width, and whether it is collapsed (desktop)
// or open as a drawer (phones). The sidebar is only a way to navigate:
// opening a session never changes its mode. Mode, width and collapse
// survive reloads on this device.

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
/** The full sidebar's width bounds in CSS pixels. */
export const sidebarWidthMin = 224;
export const sidebarWidthMax = 480;
const sidebarWidthDefault = 288;
function clampWidth(width: number) {
  return Math.round(
    Math.max(sidebarWidthMin, Math.min(sidebarWidthMax, width)),
  );
}
// stored() answers "" for a missing key; only a saved pixel width counts.
const storedWidth = z
  .string()
  .regex(/^\d{1,4}$/)
  .transform(Number)
  .safeParse(stored("sidebarWidth")).data;
let width = $state(
  storedWidth === undefined ? sidebarWidthDefault : clampWidth(storedWidth),
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
  get width() {
    return width;
  },
  /** Sets the full sidebar's width, kept within its bounds. */
  setWidth(next: number) {
    width = clampWidth(next);
    store("sidebarWidth", String(width));
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
  /** Opens the shared close, archive or remove dialog for a session. */
  close(session: SessionInfo): void;
  /** Opens the port-forwarding dialog. */
  forward(): void;
  /** Opens the add-repository dialog. */
  addRepository(): void;
  /**
   * The repository the home view is narrowed to (or the only one), whose
   * settings the sidebar footer opens; undefined elsewhere.
   */
  readonly settingsRepository: Repository | undefined;
  /** Opens the settings of `settingsRepository`. */
  openRepositorySettings(): void;
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

/** Opens a session from the sidebar or a home card; the mode stays. */
export function openSession(session: Pick<SessionInfo, "id">) {
  drawerOpen = false;
  navigate(`/s/${session.id}`);
}

/** Opens a page of the shell (a home page or a scope) and closes the drawer. */
export function openPage(path: string) {
  drawerOpen = false;
  if (location.pathname + location.hash === path) return;
  navigate(path);
}
