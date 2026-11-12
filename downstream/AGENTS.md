# Downstream Agent Guide

Read [README.md](README.md) before changing an overlay path or rebasing. It defines feature status, ownership, and removal rules.

[`ownership.json`](ownership.json) is the source of truth for the committed overlay:

- `ownedFiles` / `ownedPrefixes`: paths created and maintained by this fork;
- `mixedFiles`: upstream-owned files with downstream changes.

Update the manifest in the same commit that changes the committed overlay path set. A mixed path may appear under several features, but its classification must agree.

Before delivery, run `go test ./downstream`. After the overlay commit exists, also run:

```sh
go run ./downstream
```

The checker reads committed history only; initial bootstrap stays red until this directory and the manifest are committed together. It validates `upstreamBase` against `refs/remotes/upstream/main`; refresh that ref from canonical `mzdluo123/webmux` before a rebase, reconcile the feature list, move `upstreamBase`, and run both commands again.
