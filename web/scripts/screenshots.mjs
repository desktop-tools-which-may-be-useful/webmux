// Generates the README screenshots in docs/images/ from a real WebMux backend.
//
//   make build && pnpm --dir web screenshots
//
// It starts bin/webmux on a free port with a throwaway config, creates a demo Git
// repository with uncommitted changes, runs real shells and the `omp` agent
// (from the seeded hybrid preset, then switched to the chat view), and
// captures desktop and mobile (Pixel 7) viewports with Playwright.
//
// The omp conversation is replayed from screenshots/omp-session.jsonl by
// default: omp resumes the recorded session from an isolated agent directory,
// so no model or API key is needed. With WEBMUX_SCREENSHOTS_RECORD=1 omp asks
// a logged-in model instead, and the new session is exported to that file.
import { execFileSync, spawn } from "node:child_process";
import { once } from "node:events";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  realpathSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { createServer } from "node:net";
import { homedir, tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { fileURLToPath } from "node:url";
import { chromium, devices } from "@playwright/test";

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = resolve(webRoot, "..");
const outDir = join(repoRoot, "docs", "images");
const binary = join(repoRoot, "bin", "webmux");
const root = realpathSync(mkdtempSync(join(tmpdir(), "webmux-shots-")));
// A short, stable path keeps temp-directory noise out of the screenshots.
const demo = "/tmp/demo/bookshelf-api";
const config = join(root, "config.toml");

const record = process.env.WEBMUX_SCREENSHOTS_RECORD === "1";
const sessionFixture = join(
  webRoot,
  "scripts",
  "screenshots",
  "omp-session.jsonl",
);
// omp files sessions under its agent directory by working directory.
const ompSessions = (agentDir) =>
  join(agentDir, "sessions", demo.replaceAll("/", "-"));
const SUMMARY_PROMPT =
  "Summarize the uncommitted changes in this repo in three short bullets and point out anything risky.";
const FOLLOW_UP_PROMPT =
  "Which of those risks should I fix first? Answer in two sentences.";

// Replay gives omp its own agent directory holding only the recorded session
// and a config that skips first-run setup. Returns the omp environment and
// the session ID to resume.
function prepareReplay() {
  const agentDir = join(root, "omp-agent");
  const header = readFileSync(sessionFixture, "utf8")
    .split("\n")
    .filter(Boolean)
    .map((line) => JSON.parse(line))
    .find((entry) => entry.type === "session");
  if (header?.cwd !== demo)
    throw new Error(
      `${sessionFixture} is not an omp session recorded in ${demo}`,
    );
  const dir = ompSessions(agentDir);
  mkdirSync(dir, { recursive: true });
  const stamp = header.timestamp.replaceAll(/[:.]/g, "-");
  copyFileSync(sessionFixture, join(dir, `${stamp}_${header.id}.jsonl`));
  writeFileSync(
    join(agentDir, "config.yml"),
    "startup:\n  setupWizard: false\nmodelRoles:\n  default: anthropic/claude-opus-5-5\n",
  );
  return {
    env: {
      PI_CODING_AGENT_DIR: agentDir,
      // omp's ACP side needs a configured model to load a session, but loading
      // makes no model request. A placeholder key also keeps a real one in the
      // caller's environment from ever being used.
      ANTHROPIC_API_KEY: "webmux-screenshot-replay",
    },
    sessionId: header.id,
  };
}

// Copies the newest omp session recorded in the demo repository to the
// fixture, so later runs can replay it.
function exportSession() {
  const dir = ompSessions(
    process.env.PI_CODING_AGENT_DIR || join(homedir(), ".omp", "agent"),
  );
  const newest = readdirSync(dir)
    .filter((name) => name.endsWith(".jsonl"))
    .map((name) => join(dir, name))
    .sort((a, b) => statSync(b).mtimeMs - statSync(a).mtimeMs)[0];
  if (!newest) throw new Error(`no omp session recorded in ${dir}`);
  mkdirSync(dirname(sessionFixture), { recursive: true });
  copyFileSync(newest, sessionFixture);
  console.log("exported", sessionFixture);
}

// Points the seeded omp launch configuration at the recorded session.
async function resumeOmpConfig(baseURL, sessionId) {
  const headers = { Origin: baseURL, "Content-Type": "application/json" };
  const list = await (await fetch(`${baseURL}/api/launch-configs`)).json();
  const summary = list.find((item) => item.name === "omp");
  if (!summary) throw new Error("seeded omp launch configuration is missing");
  const url = `${baseURL}/api/launch-configs/${summary.id}`;
  const full = await (await fetch(url)).json();
  const response = await fetch(url, {
    method: "PUT",
    headers,
    body: JSON.stringify({
      name: full.name,
      icon: full.icon,
      cmd: [...full.cmd, "--resume", sessionId],
      env: full.env,
      preview: full.preview,
      chat: full.chat,
    }),
  });
  if (!response.ok)
    throw new Error(
      `update omp launch configuration: ${await response.text()}`,
    );
}

const gitEnv = {
  ...process.env,
  GIT_AUTHOR_NAME: "Rain",
  GIT_AUTHOR_EMAIL: "rain@example.com",
  GIT_COMMITTER_NAME: "Rain",
  GIT_COMMITTER_EMAIL: "rain@example.com",
};
const git = (...args) =>
  execFileSync("git", ["-C", demo, ...args], { env: gitEnv });
const write = (name, text) => {
  mkdirSync(dirname(join(demo, name)), { recursive: true });
  writeFileSync(join(demo, name), text);
};

function seedRepository() {
  rmSync(dirname(demo), { recursive: true, force: true });
  mkdirSync(demo, { recursive: true });
  git("init", "-q", "-b", "main");
  write("go.mod", "module example.com/bookshelf\n\ngo 1.26\n");
  write(
    "main.go",
    `package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /books", listBooks)
	log.Fatal(http.ListenAndServe(":8080", mux))
}
`,
  );
  write(
    "books.go",
    `package main

import (
	"encoding/json"
	"net/http"
)

type Book struct {
	ID    int    \`json:"id"\`
	Title string \`json:"title"\`
}

var books = []Book{{1, "The Go Programming Language"}, {2, "Designing Data-Intensive Applications"}}

func listBooks(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(books)
}
`,
  );
  write("README.md", "# bookshelf-api\n\nA tiny books API.\n");
  git("add", ".");
  git("commit", "-qm", "feat: initial bookshelf API");
  // Uncommitted work shown by the diff view.
  write(
    "books.go",
    `package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type Book struct {
	ID    int    \`json:"id"\`
	Title string \`json:"title"\`
	Year  int    \`json:"year,omitempty"\`
}

var books = []Book{
	{1, "The Go Programming Language", 2015},
	{2, "Designing Data-Intensive Applications", 2017},
}

func listBooks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(books); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getBook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	for _, b := range books {
		if b.ID == id {
			json.NewEncoder(w).Encode(b)
			return
		}
	}
	http.NotFound(w, r)
}
`,
  );
  write(
    "main.go",
    `package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /books", listBooks)
	mux.HandleFunc("GET /books/{id}", getBook)
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
`,
  );
  write(
    "books_test.go",
    `package main

import (
	"net/http/httptest"
	"testing"
)

func TestGetBookNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/books/99", nil)
	req.SetPathValue("id", "99")
	getBook(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
`,
  );
}

async function freePort() {
  const server = createServer();
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const { port } = server.address();
  server.close();
  await once(server, "close");
  return port;
}

async function startBackend(extraEnv) {
  const port = await freePort();
  writeFileSync(
    config,
    `listen = "127.0.0.1:${port}"\ndefault_cwd = ${JSON.stringify(demo)}\nstate_dir = ${JSON.stringify(join(root, "state"))}\n`,
  );
  const child = spawn(binary, ["--config", config], {
    cwd: demo,
    stdio: ["ignore", "pipe", "pipe"],
    env: { ...process.env, ...extraEnv, SHELL: "/usr/bin/env bash" },
  });
  let output = "";
  child.stdout.on("data", (c) => (output += c));
  child.stderr.on("data", (c) => (output += c));
  const baseURL = `http://127.0.0.1:${port}`;
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(`${baseURL}/api/sessions`)).ok)
        return { child, baseURL };
    } catch {}
    await delay(100);
  }
  throw new Error(`backend did not start:\n${output}`);
}

// Fixed colours and a short prompt keep the shell screenshots tidy.
const PROMPT =
  "export PS1='\\[\\e[1;32m\\]rain@dev\\[\\e[0m\\]:\\[\\e[1;34m\\]~/bookshelf-api\\[\\e[0m\\]$ '; clear";

// Opens the new-session dialog from the workspace overview.
async function openNewSession(page) {
  await page.goto("/");
  await page
    .getByRole("main")
    .getByRole("button", { name: "＋ 新建会话", exact: true })
    .first()
    .click();
}

// Picks an entry of the dialog's 运行 or 位置 picker; option names start
// with the entry title and continue with its command or path.
async function pick(page, field, title) {
  await page.getByRole("button", { name: field, exact: true }).click();
  const escaped = title.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  await page
    .getByRole("option", { name: new RegExp(`^${escaped}`) })
    .first()
    .click();
}

// Fills the directory and name, then creates the session.
async function finishNewSession(page, name) {
  const cwd = page.getByLabel("工作目录", { exact: true });
  if (!(await cwd.isVisible())) await pick(page, "位置", "输入或浏览目录…");
  await cwd.fill(demo);
  await page.getByRole("button", { name: "高级：会话名称" }).click();
  await page.getByLabel("会话名称", { exact: false }).fill(name);
  await page.getByRole("button", { name: "创建并进入 →", exact: true }).click();
  await page.waitForURL(/\/s\/[a-z0-9]+$/);
}

async function createSession(page, command, name) {
  await openNewSession(page);
  await pick(page, "运行", "自定义命令…");
  await page.getByLabel("自定义启动命令", { exact: true }).fill(command);
  await finishNewSession(page, name);
  await page
    .locator(".terminal-header")
    .getByRole("img", { name: /已连接/ })
    .waitFor();
  await page.locator(".xterm-helper-textarea").waitFor({ state: "attached" });
  return new URL(page.url()).pathname.split("/").at(-1);
}

// Starts a saved launch configuration (a seeded preset) on the terminal side.
async function createFromConfig(page, configName, name) {
  await openNewSession(page);
  await pick(page, "运行", configName);
  await page
    .getByRole("radiogroup", { name: "起始端", exact: true })
    .getByRole("radio", { name: "从终端开始", exact: true })
    .click();
  await finishNewSession(page, name);
  await page.locator(".xterm-helper-textarea").waitFor({ state: "attached" });
  return new URL(page.url()).pathname.split("/").at(-1);
}

// Waits until a chat turn has finished: the working indicator is gone.
async function chatIdle(page) {
  const working = page.getByRole("status").filter({ hasText: "正在工作…" });
  await working
    .first()
    .waitFor({ timeout: 30_000 })
    .catch(() => {});
  await working.first().waitFor({ state: "detached", timeout: 180_000 });
}

// Types into the terminal of `pane` (the whole page by default).
async function type(page, text, enter = true, pane = page) {
  await pane.locator(".xterm-helper-textarea").focus();
  await page.keyboard.type(text, { delay: 8 });
  if (enter) await page.keyboard.press("Enter");
}

async function shoot(page, name) {
  await delay(600);
  await page.screenshot({ path: join(outDir, name) });
  console.log("wrote", name);
}

// Draws the chart the Kitty graphics screenshot displays and returns it as
// Kitty protocol escape sequences (direct PNG transfer, 4 KiB chunks).
async function kittyChart(browser) {
  const page = await browser.newPage({ viewport: { width: 640, height: 300 } });
  await page.setContent(`<body style="margin:0;background:#0f172a;color:#e2e8f0;font:600 15px sans-serif">
    <div style="padding:16px 20px">GET /books · p95 latency (ms)</div>
    <div style="display:flex;align-items:flex-end;gap:18px;height:200px;padding:0 24px">${[
      ["v1", 82],
      ["v2", 64],
      ["v3", 41],
      ["v4", 23],
    ]
      .map(
        ([label, ms]) =>
          `<div style="flex:1;text-align:center"><div style="height:${ms * 2}px;background:linear-gradient(#34d399,#0d9488);border-radius:6px 6px 0 0"></div><div style="padding-top:6px">${label} · ${ms}</div></div>`,
      )
      .join("")}</div></body>`);
  const png = (await page.screenshot()).toString("base64");
  await page.close();
  let out = "";
  for (let i = 0; i < png.length; i += 4096) {
    const chunk = png.slice(i, i + 4096);
    const more = i + 4096 < png.length ? 1 : 0;
    const control = i === 0 ? `a=T,f=100,q=2,m=${more}` : `m=${more}`;
    out += `\x1b_G${control};${chunk}\x1b\\`;
  }
  return out;
}

// The "more screenshots" gallery of the README: split panes with a draft,
// Kitty graphics, the file explorer and the seeded launch configurations,
// each in a light and a dark desktop theme.
async function featureShots(browser, baseURL) {
  // Each screenshot starts from an empty sidebar: no sessions of earlier shots.
  const closeSessions = async () => {
    const sessions = await (await fetch(`${baseURL}/api/sessions`)).json();
    for (const { id } of sessions)
      for (const query of ["", "?remove=1"])
        await fetch(`${baseURL}/api/sessions/${id}${query}`, {
          method: "DELETE",
          headers: { Origin: baseURL },
        });
  };
  const chart = await kittyChart(browser);
  const chartFile = "/tmp/webmux-demo-chart.kitty";
  writeFileSync(chartFile, chart);
  try {
    for (const scheme of ["dark", "light"]) {
      const context = await browser.newContext({
        ...devices["Desktop Chrome"],
        viewport: { width: 1440, height: 900 },
        deviceScaleFactor: 1.25,
        baseURL,
        colorScheme: scheme,
      });
      await context.route("**/api/config", async (route) => {
        const response = await route.fetch();
        const json = { ...(await response.json()), hostname: "dev" };
        await route.fulfill({ response, json });
      });
      const page = await context.newPage();
      const featureShot = async (dir, ext) => {
        await delay(600);
        const name = `${dir}/after-desktop-${scheme}.${ext}`;
        mkdirSync(dirname(join(outDir, name)), { recursive: true });
        await page.screenshot({
          path: join(outDir, name),
          ...(ext === "jpg" ? { type: "jpeg", quality: 88 } : {}),
        });
        console.log("wrote", name);
      };

      // Split panes with a draft.
      await closeSessions();
      await createSession(
        page,
        "/usr/bin/env bash --noprofile --norc -i",
        "api",
      );
      await type(page, PROMPT);
      await delay(500);
      await type(page, "go test ./...");
      await delay(4000);
      await page.getByRole("button", { name: "草稿模式" }).click();
      await page
        .locator(".terminal-composer textarea")
        .fill("Add a POST /books endpoint that validates title and year");
      await page.getByRole("button", { name: "分屏", exact: true }).click();
      await page
        .getByRole("menuitem", { name: "左右分屏 · 新建 Shell" })
        .click();
      const second = page.locator(".split-terminal").nth(1);
      await second.locator(".xterm-helper-textarea").waitFor({
        state: "attached",
      });
      // The split shell is the login shell; switch it to the demo's bash.
      await type(page, "exec bash --noprofile --norc -i", true, second);
      await delay(800);
      await type(page, PROMPT, true, second);
      await delay(500);
      await type(page, "git diff --stat", true, second);
      await delay(1000);
      await featureShot("terminal-splits", "jpg");

      // Kitty graphics.
      await closeSessions();
      await createSession(
        page,
        "/usr/bin/env bash --noprofile --norc -i",
        "chart",
      );
      await type(page, PROMPT);
      await delay(500);
      await type(page, "echo 'p95 latency per release:'; cat " + chartFile);
      await delay(2500);
      await featureShot("kitty-graphics", "png");

      // File explorer in the code workspace.
      await closeSessions();
      await createSession(
        page,
        "/usr/bin/env bash --noprofile --norc -i",
        "files",
      );
      await type(page, PROMPT);
      await delay(500);
      await type(page, "git status -sb");
      await page
        .locator(".terminal-header")
        .getByRole("button", { name: "代码", exact: true })
        .click();
      await page.getByRole("region", { name: "代码工作区" }).waitFor();
      await page.getByRole("tab", { name: "文件", exact: true }).click();
      await page.getByRole("button", { name: "books.go", exact: true }).click();
      await page.locator("[data-code]").first().waitFor();
      await featureShot("file-explorer", "png");

      // Seeded launch configurations.
      await page.goto("/settings#launch-configs");
      await page.locator("#launch-configs").waitFor();
      await page.locator("#launch-configs").scrollIntoViewIfNeeded();
      await featureShot("cli-agents", "png");
      await context.close();
    }
  } finally {
    rmSync(chartFile, { force: true });
  }
}

async function main() {
  mkdirSync(outDir, { recursive: true });
  seedRepository();
  const featuresOnly = process.env.WEBMUX_SCREENSHOTS_ONLY === "features";
  const replay = record || featuresOnly ? undefined : prepareReplay();
  const { child, baseURL } = await startBackend(replay?.env);
  if (replay) await resumeOmpConfig(baseURL, replay.sessionId);
  const browser = await chromium.launch({
    args: ["--disable-webgl"],
    // Lets a preinstalled Chromium stand in for Playwright's own download.
    executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE || undefined,
  });
  try {
    if (featuresOnly) return await featureShots(browser, baseURL);
    const contexts = {
      desktop: await browser.newContext({
        ...devices["Desktop Chrome"],
        viewport: { width: 1440, height: 900 },
        deviceScaleFactor: 2,
        baseURL,
        colorScheme: "dark",
      }),
      mobile: await browser.newContext({
        ...devices["Pixel 7"],
        baseURL,
        colorScheme: "dark",
      }),
    };
    // Show the demo prompt's host instead of the capturing machine's hostname.
    for (const context of Object.values(contexts)) {
      await context.route("**/api/config", async (route) => {
        const response = await route.fetch();
        const json = { ...(await response.json()), hostname: "dev" };
        await route.fulfill({ response, json });
      });
    }
    const desktop = await contexts.desktop.newPage();
    const mobile = await contexts.mobile.newPage();

    // Background sessions that make the overview interesting.
    await createSession(
      desktop,
      "/usr/bin/env bash --noprofile --norc -i",
      "go test",
    );
    await type(desktop, PROMPT);
    await delay(500);
    await type(desktop, "go test ./... 2>&1 | head -5");
    await delay(3000);
    await type(
      desktop,
      "echo; echo 'Proceed with deploy? [y/N]' ; read -r answer",
    );
    await createSession(
      desktop,
      "/usr/bin/env bash --noprofile --norc -i",
      "build",
    );
    await type(desktop, PROMPT);
    await delay(500);
    await type(
      desktop,
      'for i in $(seq 1 300); do echo "compiling package $i/300"; sleep 1; done',
    );

    // omp agent session from the seeded hybrid preset, started on the TUI:
    // either resuming the recorded conversation or asking the model.
    const ompId = await createFromConfig(desktop, "omp", "omp");
    if (replay) {
      await delay(8000);
    } else {
      await delay(4000);
      await type(desktop, SUMMARY_PROMPT);
      await delay(75000);
    }
    await shoot(desktop, "desktop-omp.png");

    // Shell session.
    const shellId = await createSession(
      desktop,
      "/usr/bin/env bash --noprofile --norc -i",
      "shell",
    );
    await type(desktop, PROMPT);
    await delay(500);
    for (const command of [
      "git status -sb",
      "git log --oneline --graph --decorate",
      "git diff --stat",
      "ls",
    ]) {
      await type(desktop, command);
      await delay(700);
    }
    await shoot(desktop, "desktop-terminal.png");

    // Code workspace: Git diff.
    await desktop
      .locator(".terminal-header")
      .getByRole("button", { name: "代码", exact: true })
      .click();
    await desktop.getByRole("region", { name: "代码工作区" }).waitFor();
    await desktop.getByRole("tab", { name: "修改", exact: true }).click();
    await desktop
      .getByRole("button", { name: "修改 books.go", exact: true })
      .click();
    await desktop
      .locator(".code-diff-reader:visible [data-code]")
      .first()
      .waitFor();
    await shoot(desktop, "desktop-diff.png");

    // Session overview.
    await desktop.goto("/");
    await shoot(desktop, "desktop-overview.png");

    // Mobile: the phone takes over terminal control, so omp reflows to its width.
    await mobile.goto("/");
    await shoot(mobile, "mobile-overview.png");
    await mobile.goto(`/s/${ompId}`);
    await mobile
      .locator(".terminal-header")
      .getByRole("img", { name: /已连接/ })
      .waitFor();
    await delay(3000);
    await shoot(mobile, "mobile-omp.png");

    // Hybrid session: switch the same omp conversation to the chat view.
    await desktop.goto(`/s/${ompId}`);
    // The chat tab is named exactly 聊天 once the session is eligible.
    const chatTab = desktop
      .getByRole("tablist", { name: "会话视图", exact: true })
      .getByRole("tab", { name: "聊天", exact: true });
    await chatTab.waitFor({ timeout: 60_000 });
    await chatTab.click();
    const composer = desktop.getByRole("textbox", {
      name: "消息",
      exact: true,
    });
    await composer.waitFor({ timeout: 60_000 });
    await desktop.waitForFunction(
      () => !document.querySelector("[data-chat-view] textarea")?.disabled,
      null,
      { timeout: 60_000 },
    );
    if (replay) {
      // The recorded session already holds the follow-up turn.
      await desktop.getByRole("list", { name: "对话", exact: true }).waitFor();
      await delay(2000);
    } else {
      await composer.fill(FOLLOW_UP_PROMPT);
      await desktop.getByRole("button", { name: /^(发送|加入排队)$/ }).click();
      await chatIdle(desktop);
    }
    await shoot(desktop, "desktop-chat.png");
    await mobile.goto(`/s/${ompId}`);
    await mobile.getByRole("list", { name: "对话", exact: true }).waitFor();
    await delay(2000);
    await shoot(mobile, "mobile-chat.png");

    await mobile.goto(`/s/${shellId}`);
    await mobile
      .locator(".terminal-header")
      .getByRole("img", { name: /已连接/ })
      .waitFor();
    await delay(1500);
    await shoot(mobile, "mobile-terminal.png");
    // Phones toggle code with the header's code button, like desktop.
    await mobile
      .locator(".terminal-header")
      .getByRole("button", { name: "代码", exact: true })
      .click();
    await mobile.getByRole("region", { name: "代码工作区" }).waitFor();
    await mobile.getByRole("tab", { name: "修改", exact: true }).click();
    await mobile
      .getByRole("button", { name: "修改 books.go", exact: true })
      .click();
    await mobile
      .locator(".code-diff-reader:visible [data-code]")
      .first()
      .waitFor();
    await shoot(mobile, "mobile-diff.png");

    // Port forwarding: the demo API listens on :8080, which raises the
    // terminal's 检测到端口 banner.
    await createSession(
      desktop,
      "/usr/bin/env bash --noprofile --norc -i",
      "api",
    );
    // The code pane stays open from the diff view; give the terminal the width.
    const fold = desktop
      .locator(".terminal-header")
      .getByRole("button", { name: "折叠代码", exact: true });
    if (await fold.isVisible()) await fold.click();
    await type(desktop, PROMPT);
    await delay(500);
    await type(desktop, "go vet ./... && go test ./...");
    await delay(5000);
    await type(desktop, "go run .");
    await desktop.getByText(/^检测到端口 8080（/).waitFor({ timeout: 120_000 });
    await shoot(desktop, "desktop-port-forward.png");

    // Quick Start (/new) needs a registered repository to start from.
    const registered = await fetch(`${baseURL}/api/repositories`, {
      method: "POST",
      headers: { Origin: baseURL, "Content-Type": "application/json" },
      body: JSON.stringify({ path: demo }),
    });
    if (!registered.ok)
      throw new Error(`register repository: ${await registered.text()}`);
    await desktop.goto("/new");
    await desktop
      .getByRole("button", { name: "仓库：bookshelf-api", exact: true })
      .waitFor();
    await desktop
      .getByRole("textbox", { name: "任务描述", exact: true })
      .fill("Add a POST /books endpoint that validates the title and year");
    // The agent options load asynchronously; capture them once loaded.
    await desktop
      .getByText("正在读取配置…")
      .waitFor({ state: "detached", timeout: 30_000 })
      .catch(() => {});
    await shoot(desktop, "desktop-quick-start.png");
    await featureShots(browser, baseURL);
    if (record) exportSession();
  } finally {
    await browser.close();
    child.kill("SIGTERM");
    rmSync(root, { recursive: true, force: true });
    rmSync(dirname(demo), { recursive: true, force: true });
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
