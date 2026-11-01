# Code Conventions and Common Patterns

Detail moved out of `AGENTS.md`, which has a 10 KiB budget. The short versions in `AGENTS.md` are the rules you need on every change; read this page when you touch the area below.

## Go

- Follow [the Go quality baseline](../go-style.md), based on the Uber Go Style Guide with documented local exceptions.
- New Go code must add no lint warnings. Use the pinned golangci-lint release and `make go-lint`.
- Go uses gofmt.
- Return contextual Go errors and API JSON errors with meaningful HTTP statuses.
  - Services report sentinel error kinds (for example `workspace.ErrConflict`).
  - Only `internal/api/errors.go` maps them to statuses.
- Preserve cancellation and deadlines for PTY writes, and join spawned connection goroutines.
- Protect manager/session state with existing locks. Do not hold them across blocking network or PTY operations.
- Shared output chunks stay immutable. Exit publication follows PTY drain and process cleanup.

## Commands, launch, and security

- Execute commands as argv, not shell-expanded strings.
- Session creation takes either a saved custom launch `configId` or an explicit full `cmd`/`env`. They are mutually exclusive.
- There are no built-in launch presets or implicit shell wrappers.
  - Loading `.bashrc` requires an explicit command such as `bash -ic 'exec opencode "$@"' opencode`.
- Preserve authentication and Origin checks on API/WebSocket paths.
- Never log terminal input/output. Never expose tokens through configuration responses.

## Wire contract

- Backend payloads and `web/src/schemas.ts` form one contract.
- Every JSON body and frame the controller or hub encodes or decodes is a type in `internal/contract`, converted from service types in `internal/wire`.
- Services emit typed `session.Message` values and never encode client-visible JSON.
- Keep frame handling in `internal/hub` and `web/src/hooks/terminal-socket.svelte.ts` (composed by `useTerminal` in `terminal.svelte.ts`) aligned.
- When changing JSON fields, update schemas, contract types, and consumers together.
- Re-record `internal/api/testdata/wire` with `go test ./internal/api -run TestWireSnapshots -update`.

## Frontend

- Frontend files use Oxfmt with an 80-column print width, two spaces, semicolons, trailing commas, and Svelte formatting enabled.
- Oxlint checks JS/TS and Svelte script blocks, with type-aware Promise rules for `.ts` and `.svelte.ts`. Keep `svelte-check` for Svelte template and type diagnostics.
- TypeScript is strict with `noUncheckedIndexedAccess`. `$lib` aliases `web/src/lib`.
- Follow existing names: PascalCase Svelte components, camelCase TypeScript functions, `*Schema` Zod contracts, `*.svelte.ts` rune-bearing hooks, Go `*_test.go`, and Playwright `*.spec.ts`.
- Before writing UI, look for an existing component in `web/src/lib/components/ui/`, then `web/src/components/`, then the shadcn-svelte registry.
  - Give design-system components a new variant or size instead of restyling them with `class`.
- Run `pnpm --dir web check:arch` before finishing frontend work.
  - The committed baselines are empty, so any violation fails. Fix it rather than adding `eslint-disable`/`jscpd:ignore` or growing a baseline.
  - A baseline is only for adopting a new rule that has existing violations, followed by `pnpm --dir web arch:ratchet` as they are fixed.
  - See [Frontend Architecture Guard](../frontend-architecture.md).
- Icons: use Iconify for new or changed UI icons, preferring the `tabler` collection for visual consistency.
  - Bundle the required icon data locally so icons work without a public icon API.
  - Migrate existing direct icon-library imports when updating the related UI.
  - Icon-only controls must have an accessible name and a visible tooltip. Decorative icons use `aria-hidden="true"`. See [Icons](../design-system.md#icons).
- State and effects, per [Frontend Code Quality](../frontend-style.md):
  - `$state` only for state that drives rendering, and `$derived` for computed values.
  - `$effect` only for side effects with cleanup.
  - Stable business IDs for keyed lists, typed `$props()`, and Svelte 5 event attributes (`onclick`) rather than `on:` directives.
  - Use lifecycle cleanup for sockets, xterm, listeners, and observers.
- Reuse `api.ts` and `ws.ts`. Do not add parallel request or reconnect conventions.
- Validate HTTP responses, WebSocket JSON, and local settings with Zod before updating state. Derive types with `z.infer`.
  - High-risk request bodies use the optional body schema in `api()` and are validated before any network request.
  - PTY data stays binary, not JSON or decoded text.
- Frontend failures use `APIError`. HTTP 401 triggers `webmux:auth`.

## Development commands

Run from the repository root.

- `pnpm --dir web install --frozen-lockfile`: install locked frontend dependencies.
- `make web`: install dependencies, run Svelte/TypeScript checks, and build `web/dist`.
- `make build`: build the frontend first, then the embedded Go executable `bin/webmux`.
- `make release`: build Linux and macOS (darwin) amd64 and arm64 binaries.
- `./bin/webmux --listen 127.0.0.1:7681`: run the backend and embedded production app.
- `pnpm --dir web dev`: run Vite at `http://127.0.0.1:5173`.
- `pnpm --dir web check`: check Svelte/TypeScript without building.
- `pnpm --dir web check:test`: type-check `web/src` and top-level `web/test/*.test.ts` with Svelte support.
- `make check`: build the frontend, check test types and formatting, run the doc guard, Oxlint, the architecture guard, `go vet` and `golangci-lint`.
- `make docs-check`: run the documentation/spec size and tree-layout guard (`internal/docguard`).
- `pnpm --dir web lint`: run Oxlint, including type-aware Promise rules, on frontend code.
- `pnpm --dir web check:arch`: run the frontend architecture guard (ESLint design-system rules, dependency-cruiser, jscpd, knip, architecture test).
- `make format` / `make format-check`: apply/check gofmt and Oxfmt on configured targets.

Vite proxies `/api` and `/ws` to backend port 7681.
Run the backend separately and add `allowed_origins = ["http://127.0.0.1:5173"]` to its development TOML. Otherwise browser write requests are rejected.
Configuration defaults to `~/.config/webmux/config.toml`. `-config`, `-listen`, and `-token` override startup settings.
