import type { CodeHostState } from "./schemas";
import type { TextInsertion } from "./terminal-draft.svelte";

/** What a code workspace needs from the terminal page that hosts it. */
export interface CodeHost extends Readonly<CodeHostState> {
  /** A path another view asks to show, such as a chat tool call's diff. */
  readonly reveal?: { path: string; key: number };
  paste(text: string): Promise<TextInsertion>;
}
