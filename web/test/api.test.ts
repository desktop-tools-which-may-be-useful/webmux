import { afterEach, expect, test, vi } from "vitest";
import { z } from "zod";
import { api } from "../src/api";
import { answerSwitch, changeView } from "../src/chat-api";
import { CreateSessionRequestSchema } from "../src/schemas";

vi.stubGlobal("localStorage", { getItem: () => null });
afterEach(() => {
  vi.unstubAllGlobals();
});

test("an invalid request body is rejected before any network request", async () => {
  const fetchMock = vi.fn<typeof fetch>();
  vi.stubGlobal("fetch", fetchMock);
  await expect(
    api(
      "/sessions",
      z.undefined(),
      "POST",
      { configId: "saved", cmd: ["sh"] },
      undefined,
      z.object({ configId: z.string() }).strict(),
    ),
  ).rejects.toThrow();
  expect(fetchMock).not.toHaveBeenCalled();
});

test("view changes advertise turn-choice support outside the strict body", async () => {
  vi.stubGlobal("localStorage", { getItem: () => null });
  const fetchMock = vi
    .fn<typeof fetch>()
    .mockImplementation(
      async () =>
        new Response(
          JSON.stringify({ view: "terminal", dropped: [], unsupported: [] }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
    );
  vi.stubGlobal("fetch", fetchMock);

  await changeView("session1", "chat");
  await answerSwitch("session1", "cancel");

  expect(fetchMock.mock.calls.map(([path]) => path)).toEqual([
    "/api/sessions/session1/view?turnChoice=1",
    "/api/sessions/session1/view",
  ]);
  const initial = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
  expect(initial).toMatchObject({ to: "chat" });
  expect(initial).not.toHaveProperty("turnChoice");
});

test.each([{ cmd: [] }, { env: {} }])(
  "an empty override does not conflict with configId: %j",
  (override) => {
    const body = { configId: "saved", ...override };
    expect(CreateSessionRequestSchema.parse(body)).toEqual(body);
  },
);

test("non-empty command or environment conflicts with configId", () => {
  for (const body of [
    { configId: "saved", cmd: ["sh"] },
    { configId: "saved", env: { A: "1" } },
  ])
    expect(CreateSessionRequestSchema.safeParse(body).success).toBe(false);
});

test("the initial terminal size stays within the PTY's 1..1000 grid", () => {
  const cmd = ["sh"];
  expect(
    CreateSessionRequestSchema.safeParse({ cmd, cols: 200, rows: 50 }).success,
  ).toBe(true);
  for (const size of [
    { cols: 0, rows: 24 },
    { cols: 1001, rows: 24 },
    { cols: 80, rows: 1.5 },
  ])
    expect(CreateSessionRequestSchema.safeParse({ cmd, ...size }).success).toBe(
      false,
    );
});
