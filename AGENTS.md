# AGENTS.md — Daiku CLI

This file is the operating guide for agents working in this repository.

## Non-negotiable workflow

- Use a dedicated Git worktree and one branch per Fizzy card. Never share a checkout with another agent.
- Respect the card's declared base. Dependent work uses stacked branches and stacked pull requests.
- Do not merge pull requests. A separate agent reviews; a human merges.
- Do not force-push, bypass hooks or checks, create releases, or broaden the card's scope.
- Preserve unrelated changes. Ask the orchestrator before editing files reserved by another active card.

## Product contract

- Commands, flags, JSON fields, and machine error codes are English.
- `--json` uses the stable envelopes documented in `README.md`; errors go to stderr and process exit codes stay meaningful.
- Piped output is deterministic and contains no ANSI escapes, prompts, spinners, or timing-dependent text.
- Human TTY output may add concise styling, but must preserve the same semantics.
- Support macOS and Linux on amd64 and arm64. Avoid CGO unless a card explicitly requires it.

## Code recipes

### Add a command domain

1. Create `internal/commands/<domain>/`.
2. Implement `cli.Module`; let the package own its complete Cobra subtree.
3. Inject clients, credential stores, clocks, and terminal behavior. Do not use mutable globals or `init()` registration.
4. Add the module at the composition root in `cmd/daiku/main.go`. Coordinate this small integration edit with stacked PR order.
5. Wrap Cobra positional validators with `cli.UsageArgs` so their failures are typed as safe usage errors. Flag parsing and required flags are typed at the app boundary; never infer error type from message text.
6. Test human and JSON output, invalid input, exit codes, and pipe behavior.

### Return errors

Return a `*cli.Error` with a stable snake_case code, user-safe message, documented `cli.ExitCode`, and optional structured details. Never print an error and also return it; the app boundary owns error rendering.

### Emit output

Use `cli.WriteSuccess` for machine output. Write human output through `command.OutOrStdout()` so tests and callers can redirect it. Never print directly to `os.Stdout` from a command.

## Verification

Before handing off a commit, run:

```sh
make check
make cross-build
git diff --check
```

Report the exact commands and results to the orchestrator. CI repeats formatting, vet, race tests, and all four target builds.
