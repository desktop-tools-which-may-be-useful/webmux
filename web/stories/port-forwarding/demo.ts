// Demo data for the port-forwarding stories. The real components read the
// forwards store and call /api/forwards; `serveForwards` answers those calls
// in memory and feeds the store the way the session-list connection does.
import {
  defaultForwardOptions,
  receiveSessionFrame,
} from "../../src/forwards.svelte";
import {
  ForwardRequestSchema,
  ForwardSchema,
  ForwardUpdateSchema,
  SessionInfoSchema,
  type Forward,
  type ListeningPort,
} from "../../src/schemas";
import { session } from "../fixtures";

export function demoForward(
  port: number,
  fields: Partial<Omit<Forward, "port" | "path">> = {},
): Forward {
  return ForwardSchema.parse({
    port,
    path: `/forward/${port}/`,
    label: "",
    session: "",
    process: "",
    options: defaultForwardOptions,
    status: "listening",
    createdAt: "2026-01-01T10:00:00Z",
    ...fields,
  });
}

export function demoSession(
  id: string,
  title: string,
  listeningPorts: ListeningPort[],
) {
  return SessionInfoSchema.parse({
    ...session,
    id,
    title,
    cwd: "~/projects/shop",
    status: "running",
    attention: null,
    reportedCwd: null,
    listeningPorts,
    state: "active",
    views: ["terminal"],
    view: "terminal",
    transition: null,
    agent: null,
  });
}

export const demoSessions = [
  demoSession("webwebwe", "web", [{ port: 5173, process: "node" }]),
  demoSession("storyboo", "storybook", [{ port: 6006, process: "node" }]),
  demoSession("apiserve", "api-server", [{ port: 9229, process: "node" }]),
];

export const demoForwards = [
  // Vite started with --base /forward/5173/ keeps the prefix; the static
  // docs preview uses the default, which strips it.
  demoForward(5173, {
    session: "webwebwe",
    process: "node",
    options: { ...defaultForwardOptions, prefix: "keep" },
  }),
  demoForward(8000, { label: "docs 静态预览", status: "unreachable" }),
];

let forwards: Forward[] = [];
function publish(next: Forward[]) {
  forwards = next;
  receiveSessionFrame(JSON.stringify({ type: "forwards", forwards }));
}
function reply(status: number, body?: unknown) {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/**
 * Seeds the store with `initial` and answers /api/forwards in memory until
 * the returned function restores fetch. `fail` rejects every new forward
 * with that server message.
 */
export function serveForwards(initial: Forward[], fail = ""): () => void {
  publish(initial);
  const original = window.fetch;
  window.fetch = async (input, init) => {
    const url = new URL(
      input instanceof Request ? input.url : String(input),
      location.href,
    );
    const match = /^\/api\/forwards(?:\/(\d+))?$/.exec(url.pathname);
    if (!match) return original(input, init);
    const method = init?.method ?? "GET";
    const body: unknown = init?.body ? JSON.parse(String(init.body)) : {};
    if (method === "POST" && !match[1]) {
      if (fail) return reply(409, { error: fail });
      const request = ForwardRequestSchema.parse(body);
      const created = demoForward(request.port, {
        label: request.label ?? "",
        session: request.session ?? "",
        process: request.process ?? "",
        ...(request.options && { options: request.options }),
      });
      publish([...forwards, created]);
      return reply(201, created);
    }
    const port = Number(match[1]);
    const existing = forwards.find((forward) => forward.port === port);
    if (!existing) return reply(404, { error: "转发不存在" });
    if (method === "DELETE") {
      publish(forwards.filter((forward) => forward.port !== port));
      return reply(204);
    }
    const changed = { ...existing, ...ForwardUpdateSchema.parse(body) };
    publish(
      forwards.map((forward) => (forward.port === port ? changed : forward)),
    );
    return reply(200, changed);
  };
  return () => {
    window.fetch = original;
  };
}
