# Downstream Overlay

This directory isolates fork policy from upstream WebMux files. The overlay is one reviewed commit range from the pinned upstream base in [`ownership.json`](ownership.json) to `HEAD`.

## Features

| ID | Feature | Status | Removal condition |
| --- | --- | --- | --- |
| `WM-DS-01` | External ACP agent declarations | `active` | Upstream includes an equivalent supported declaration mechanism and regressions. |
| `WM-DS-02` | Tri-state terminal→chat switching | `pending-upstream` | Upstream includes equivalent behavior and regressions. |
| `WM-DS-03` | Downstream ownership governance | `active` | The repository no longer carries a downstream overlay. |

## WM-DS-01

**External ACP agent declarations.**

The committed overlay adds strict, bounded `WEBMUX_ACP_AGENTS` declarations for ACP agents without built-in adapters. It owns the new parser, external-agent profile, process regression, and shared launch-icon package. Integration still patches upstream command wiring, launch/hybrid services, tests, architecture inventory, and user documentation.

## WM-DS-02

**Tri-state terminal→chat switching.**

The committed fix distinguishes a busy terminal turn, an idle turn, and an unknown heuristic wait. It preserves structured blocked/question/permission interventions during startup quiet, serializes switch answers through the actor so the first answer wins, and keeps CLI and frontend choice phases aligned. Current clients opt in to the new unknown-state choice phase on the initial switch request; clients without that capability retain the interrupt-first flow and never receive an unknown phase. Its `pending-upstream` status means the behavior is active downstream while an equivalent upstream implementation is still absent.

Its two fork-owned regression/implementation files and every upstream-owned integration path are classified under `WM-DS-02` in `ownership.json`. A mixed path may be listed by several features when the classification agrees.

Remove this feature only when upstream includes equivalent behavior and regressions, then delete its overlay hunks and manifest entry in the same rebase or commit.

## WM-DS-03

**Downstream ownership governance.**

The governance feature owns this directory, its independent CI workflow, and the root agent-guide bridge. It keeps downstream lifecycle and ownership rules outside upstream-owned architecture and documentation guards.

## Ownership model

[`ownership.json`](ownership.json) classifies every path changed between `upstreamBase` and `HEAD`:

- `ownedFiles` names files absent at the pinned upstream base and maintained by this fork; `ownedPrefixes` covers a reserved downstream namespace.
- `mixedFiles` names files that exist upstream and contain downstream changes. The file remains upstream-owned; only the overlay hunk is downstream-specific.

The manifest tracks committed history, not the working tree. During this initial bootstrap, the repository-level check is expected to fail until the governance files and manifest are committed together because `HEAD` has no manifest. After bootstrap, dirty edits remain intentionally invisible; their path classifications become enforceable as soon as they are committed.

## Update workflow

1. Read [`AGENTS.md`](AGENTS.md) before changing an owned or mixed path.
2. Keep feature prose and lifecycle decisions in this file. Keep the agent guide short.
3. Add or remove manifest paths atomically with the commit that changes the overlay path set. Keep feature IDs and path arrays sorted.
4. Run `go test ./downstream`. These tests use temporary Git repositories and also prove that dirty files are ignored while an unclassified committed file fails.
5. Run `go run ./downstream` against committed history. A clean result means the pinned base belongs to the trusted `refs/remotes/upstream/main` history, is an ancestor of `HEAD`, and every committed overlay path has one consistent classification.

## Rebase workflow

1. Decide whether each downstream feature remains, moves upstream, or is removed under its stated condition.
2. Rebase onto the new upstream commit and update `upstreamBase`.
3. Resolve `mixedFiles` as upstream-owned files; reapply only the downstream behavior that remains necessary.
4. Reconcile every added/modified path in `ownership.json`, then run both checks.
5. Review the resulting upstream-base-to-HEAD diff. Do not use broad globs, whole-file hashes, or merge drivers that hide upstream changes.
