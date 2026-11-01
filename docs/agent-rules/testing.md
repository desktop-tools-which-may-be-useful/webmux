# Testing, QA, and UI Pull Requests

Detail moved out of `AGENTS.md`, which has a 10 KiB budget.

## Commands

```sh
make test                         # Full gate: Go race + all frontend suites
make test-fast                    # Go race + Vitest unit/browser (no Storybook/E2E/perf)
make test-unit                    # Vitest in Node
make test-browser                 # Vitest in headless Chromium
make test-e2e                     # Playwright: desktop + Pixel 7 Chromium
make test-browsers-install        # Chromium (+ Linux libraries; may require sudo)
```

- Target a frontend test with `pnpm --dir web exec vitest run --config vitest.config.ts test/input.test.ts`.
- Target an E2E spec with `pnpm --dir web exec playwright test test/e2e/terminal.spec.ts`.
- Target a Go package with `go test -race ./internal/session`.

## What to cover

- Go tests use the standard `testing` package.
- Linux/macOS lifecycle tests cover child-process cleanup, waiting detection, and input cancellation. Subreaper reaping is Linux-only.
- macOS temp paths sit behind `/var -> /private/var`. Compare against canonical paths: packages' `TestMain` canonicalizes `TMPDIR`, and E2E uses `realpath`.
- Preserve behavioral coverage for authentication/Origin rejection, argv boundaries, replay continuity, exit ordering, cancellation, terminal input/rendering, and touch interactions.
- No numeric coverage threshold is configured.
- Chromium mobile emulation does not replace real-device iOS Safari/Android keyboard and native selection-menu checks.

## Test runtime

- Browser tests exercise real Svelte/Tailwind/xterm rendering without a backend.
- Local `make test` builds one production frontend and `.cache/webmux-e2e/webmux` once.
  - It also builds the fake ACP agent (`internal/acp/testdata/fakeagent`) as `.cache/webmux-e2e/agents/{omp,opencode}`.
  - E2E/performance reuse them via `WEBMUX_E2E_BINARY`. Agents are found in `agents/` beside it.
  - `make test-fast` skips Storybook, E2E and performance.
- Workers share only the immutable executables. Each has an independent backend, HOME/config/workdirs, fake-agent session stores (the agents come first on each backend's `PATH`) and dynamic ports.
  - Up to two workers locally, two on Ubuntu CI and four on macOS CI. `WEBMUX_E2E_WORKERS` overrides.
  - Session, process and hybrid-record cleanup is automatic.
- Do not connect tests to an existing developer service.

## CI

- CI builds the Linux and macOS E2E executables and fake agents once in the Ubuntu Go job, before its race tests (`.github/scripts/package-e2e-runtime.mjs`).
  - They are shared, with the frontend output and locked Playwright runtime, through a same-run artifact.
- `.github/workflows/test.yml` runs parallel jobs:
  - Ubuntu: formatting/architecture guard/frontend build, Go race, Vitest, Storybook, single-worker 4x CPU performance, and four E2E shards (desktop and mobile projects).
  - E2E shards prepare Node while waiting for the artifact, reuse the executable and Playwright packages, and skip pnpm/frontend dependencies and Go setup.
  - macOS: native Go race (excluding the static guards `archtest`/`docguard` and the Windows `tray`/`wsl`/`wslclient` packages) using the shared frontend output, and two desktop-project E2E shards on separate runners.
  - `Test result` aggregates every job, and all results are required by the final gate.
- Every browser job installs Chromium through `.github/actions/playwright`, which caches the Linux apt packages because a slow Ubuntu mirror can otherwise consume a job's whole timeout.
- PR runs restore caches read-only and superseded PR runs are cancelled. Caches are saved from `main`.
- Failure diagnostics live in `web/playwright-report/`, `web/test-results/`, and `web/.vitest/attachments/` (uploaded per job). Open the E2E HTML report with `pnpm --dir web test:e2e:report`.

## UI pull request evidence

- PRs that change visible UI (layout, styling, components, or interaction states) MUST embed screenshots of the final implementation in the PR description.
  - Capture the running app or its real Storybook components. Mockups and test summaries do not replace screenshots.
- Show before/after views for changes to an existing interface.
  - Include the affected light/dark themes and desktop/mobile layouts when the change spans them.
  - Label the viewport and any sample data.
- Use GitHub attachments or committed image URLs that reviewers can open. Local filesystem paths and localhost URLs are not valid PR image links.
- Check that the images render in the published PR and refresh them when later changes alter the UI.
- Use isolated fixtures or demo sessions without credentials or private terminal output.
