import { SvelteMap } from "svelte/reactivity";
import { SessionInfoSchema } from "../../src/schemas";
import { session as base } from "../fixtures";
import type { Entry } from "./data";

// Manual titles set through the real rename dialog in the prototype. The
// real sidebar keeps no titles: they arrive with the session list.
const manual = new SvelteMap<string, string>();

export const titleOf = (entry: Entry) => manual.get(entry.id) ?? entry.title;

/**
 * Answers the dialog's `PATCH /api/sessions/{id}` in place of the server:
 * a non-empty name becomes the manual title, an empty one hands the title
 * back, an invalid one is refused. Every other request passes through.
 * Returns the function that restores the original fetch.
 */
export function interceptRename(): () => void {
  const original = window.fetch;
  window.fetch = async (input, init) => {
    const url = typeof input === "string" ? input : input.toString();
    const match = /\/api\/sessions\/([^/?]+)$/.exec(url);
    if (!match || init?.method !== "PATCH") return original(input, init);
    const { name } = JSON.parse(String(init.body)) as { name: string };
    // Control characters or more than 512 bytes, as the server refuses.
    const refused =
      [...name].some(
        (c) => c.charCodeAt(0) < 0x20 || c.charCodeAt(0) === 0x7f,
      ) || new TextEncoder().encode(name).length > 512;
    if (refused)
      return new Response(
        JSON.stringify({ error: "标题不能含控制字符，且不能超过 512 字节" }),
        { status: 400, headers: { "Content-Type": "application/json" } },
      );
    const id = decodeURIComponent(match[1]!);
    if (name) manual.set(id, name);
    else manual.delete(id);
    return new Response(
      JSON.stringify(
        SessionInfoSchema.parse({ ...base, title: name || base.title }),
      ),
      { headers: { "Content-Type": "application/json" } },
    );
  };
  return () => {
    window.fetch = original;
  };
}
