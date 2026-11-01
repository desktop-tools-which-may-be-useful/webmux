# Agent Rules

`AGENTS.md` is loaded into every agent session, so it has a hard 10 KiB budget enforced by `internal/archtest` (`TestAgentsGuideSize`). It keeps only the rules needed on almost every change. Everything else lives here.

When `AGENTS.md` outgrows the budget, move the least frequently needed rules into a page below (or a new one), leave a one-line pointer in `AGENTS.md`, and keep the rule text intact. Do not delete rules or raise the limit to make the test pass.

## Pages

- [Repository Map](agent-rules/repo-map.md): key directories, important files, runtime and tooling versions.
- [Code Conventions and Common Patterns](agent-rules/conventions.md): Go, command/launch security, wire contract, frontend rules, and development commands.
- [Testing, QA, and UI Pull Requests](agent-rules/testing.md): test commands, coverage expectations, test runtime, CI layout, and screenshot evidence.
- [User Stories Maintenance](agent-rules/user-stories-maintenance.md): when and how to keep `docs/user-stories*` in sync.

## Related

- [Architecture](architecture.md) and its layer rules R0–R10.
- [Documentation governance](doc-governance.md) for size and tree-layout limits.
- [Go style](go-style.md), [Frontend style](frontend-style.md), [Design system](design-system.md).
