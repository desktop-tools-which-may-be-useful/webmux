import { test, expect } from "@playwright/test";
import { startE2EServer, type E2EServer } from "./server.mjs";
import { revealSidebar } from "./fixtures";

// The gateway asks for authentication only when WebMux has a token.
const token = "e2e-port-forwarding";
const auth = { Authorization: `Bearer ${token}` };
let server: E2EServer;

test.beforeAll(async () => {
  server = await startE2EServer({ token });
});

test.afterAll(async () => {
  await server?.close();
});

// A dev-server stand-in run as the session's own process: any path serves a
// page, and a WebSocket upgrade echoes short text frames back.
const echoServer = String.raw`
const http = require("node:http");
const { createHash } = require("node:crypto");
const server = http.createServer((request, response) => {
  response.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
  response.end("<!doctype html><title>Echo</title><h1>Echo ready</h1>");
});
server.on("upgrade", (request, socket) => {
  const accept = createHash("sha1")
    .update(request.headers["sec-websocket-key"] + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
    .digest("base64");
  socket.write("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + accept + "\r\n\r\n");
  socket.on("data", (frame) => {
    if ((frame[0] & 0x0f) === 8) return socket.end();
    const length = frame[1] & 0x7f;
    const mask = frame.subarray(2, 6);
    const payload = Buffer.from(frame.subarray(6, 6 + length).map((byte, i) => byte ^ mask[i % 4]));
    socket.write(Buffer.concat([Buffer.from([0x81, payload.length]), payload]));
  });
});
server.listen(0, "127.0.0.1", () => console.log("ECHO_READY " + server.address().port));
`;

test("a detected port forwards, opens through authentication, echoes WebSockets and is removed", async ({
  page,
  context,
  request,
}) => {
  const created = await request.post(`${server.baseURL}/api/sessions`, {
    headers: auth,
    data: {
      cwd: server.workDir,
      cmd: [process.execPath, "-e", echoServer],
      name: "echo",
    },
  });
  expect(created.status()).toBe(201);
  const { id } = await created.json();
  let port = 0;
  await expect
    .poll(
      async () => {
        const session = await request.get(
          `${server.baseURL}/api/sessions/${id}`,
          { headers: auth },
        );
        port = (await session.json()).listeningPorts[0]?.port ?? 0;
        return port;
      },
      { message: "The session reports its listening port", timeout: 15_000 },
    )
    .toBeGreaterThan(0);

  await page.goto(`${server.baseURL}/#token=${encodeURIComponent(token)}`);
  await expect(
    page
      .getByRole("main")
      .getByRole("button", { name: "＋ 新建会话", exact: true })
      .first(),
  ).toBeVisible();
  await page.goto(`${server.baseURL}/s/${id}`);
  const prompt = page.getByText(new RegExp(`^检测到端口 ${port}（`));
  await expect(prompt).toBeVisible();

  const navigations: string[] = [];
  context.on("request", (sent) => {
    if (sent.isNavigationRequest()) {
      const url = new URL(sent.url());
      navigations.push(url.pathname + url.search);
    }
  });
  const [popup] = await Promise.all([
    context.waitForEvent("page"),
    page.getByRole("button", { name: "转发并打开" }).click(),
  ]);
  await expect(
    popup.getByRole("heading", { name: "Echo ready" }),
  ).toBeVisible();
  expect(popup.url()).toBe(`${server.baseURL}/forward/${port}/`);
  expect(navigations).toContain(
    `/forward-auth?next=${encodeURIComponent(`/forward/${port}/`)}`,
  );
  // The ticket travels in a form body, never in an address.
  expect(navigations.filter((path) => path.includes("code="))).toEqual([]);
  await expect(prompt).toBeHidden();

  const echoed = await popup.evaluate(() => {
    const { promise, resolve, reject } = Promise.withResolvers<string>();
    const socket = new WebSocket(
      `ws://${location.host}${location.pathname}socket`,
    );
    socket.onopen = () => socket.send("ping through the gateway");
    socket.onmessage = (event) => {
      resolve(String(event.data));
      socket.close();
    };
    socket.onerror = () => reject(new Error("echo socket failed"));
    return promise;
  });
  expect(echoed).toBe("ping through the gateway");

  await page.goto(`${server.baseURL}/`);
  await revealSidebar(page);
  await page.getByRole("button", { name: "端口转发（1 个）" }).click();
  const dialog = page.getByRole("dialog", { name: "端口转发" });
  await dialog.getByRole("button", { name: `移除端口 ${port} 的转发` }).click();
  await expect(dialog.getByText("还没有转发")).toBeVisible();
  await page.keyboard.press("Escape");
  await revealSidebar(page);
  await expect(
    page.getByRole("button", { name: "端口转发", exact: true }),
  ).toBeVisible();
  const removed = await popup.reload();
  expect(removed?.status()).toBe(404);
});
