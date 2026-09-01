# Contributing

## Local checks

Install Go 1.25 or newer, then run:

```sh
make check
make cross-build
```

Pull requests must keep `go test -race ./...`, `go vet ./...`, formatting, and Darwin/Linux amd64/arm64 builds green.
CI runs compatibility tests with Go 1.25.0. Cross-builds, release validation,
security scans, and release artifacts use Go 1.26.7.

## Structure

- `cmd/daiku/` is the thin executable composition root.
- `internal/cli/` owns process lifecycle, output envelopes, exit codes, and module contracts.
- `internal/commands/<domain>/` owns a top-level command domain and its tests.

Add behavior behind a small interface and inject dependencies. Do not add package-level mutable state, registration through `init()`, or API calls directly in Cobra presentation code. Commands and JSON fields are English. Human-facing copy should remain clear without ANSI styling.

## Worktrees and stacked pull requests

Every card is implemented in its own Git worktree and branch. Never run two agents in the same checkout. Branches use `feat/fizzy-<card>-<slug>` (or `fix/...`) and identify their Fizzy card in the pull request.

Keep stacks narrow and dependency ordered:

1. Branch the first card from `main`.
2. Branch dependent cards from the previous stack branch.
3. Open one focused PR per card and state its base and dependency.
4. Rebase the remaining stack after a human merges its base.
5. Never merge, force-push, or bypass checks on an agent's behalf.

Parallel cards should own separate command-domain packages. If two cards need to change the executable composition root, sequence that small integration change in stack order.

## Pull request handoff

Include the Fizzy card, stack position, contract/schema commit when relevant, tests run, risk, and rollback notes. A separate agent reviews the diff; a human performs the merge.
