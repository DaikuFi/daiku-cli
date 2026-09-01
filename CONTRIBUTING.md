# Contributing

This guide covers local development, architecture, generated contracts, agent integrations, and pull request handoff. User setup and command examples belong in [README.md](README.md). Release operations belong in [RELEASE.md](RELEASE.md).

## Set up local development

Install Go 1.25 or newer, clone the repository, and build the binary:

```sh
git clone git@github.com:DaikuFi/daiku-cli.git
cd daiku-cli
make build
./bin/daiku --help
```

Release artifacts and security scans use Go 1.26.7.

Run the standard checks before opening a pull request:

```sh
make check
make cross-build
```

Pull requests must keep formatting, `go vet ./...`, `go test -race ./...`, and Darwin/Linux amd64/arm64 builds green. Continuous integration tests compatibility with Go 1.25.0.

## Follow the command architecture

Each command domain implements `cli.Module` in its own package and owns its Cobra subtree:

- `cmd/daiku/` is the thin executable composition root.
- `internal/cli/` owns process lifecycle, output envelopes, exit codes, and module contracts.
- `internal/commands/domain_name/` owns one top-level command domain and its tests.

The executable injects modules explicitly:

```go
app := cli.New(
	cli.WithModule(versioncommand.New(version)),
	cli.WithModule(accountscommand.New(client)),
)
```

Add behavior behind a small interface and inject dependencies. Do not add package-level mutable state, register commands through `init()`, or call the API directly from Cobra presentation code. Commands and JSON fields remain in English. Human-facing copy must remain clear without terminal styling.

## Preserve the machine-output contract

Interactive output may use ANSI styling when stdout is a terminal. Redirected output never includes ANSI escapes. Scripts should pass `--json`. Agents should pass `--agent`.

Agent mode implies `--json` and `--no-input`, disables terminal behavior, and adds deterministic `breadcrumbs` to every success or error envelope. A command that requires interaction returns `interaction_required`. A command that fails to emit the stable envelope returns `agent_output_invalid`.

`commands --json` returns every visible Cobra command once, including usage, flags, required and inherited markers, and direct subcommands. Structured help returns the same metadata for one command. `--no-input` can also accompany human output when a caller must guarantee that stdin is not read.

Successful JSON uses this envelope:

```json
{"ok":true,"data":{"version":"0.1.0"}}
```

Errors go to stderr and use this envelope:

```json
{"ok":false,"error":{"code":"usage_error","message":"unknown command \"example\" for \"daiku\""}}
```

The process exit code remains authoritative:

| Exit | Meaning | Stable error category |
| ---: | --- | --- |
| 0 | Success | Not applicable |
| 1 | Internal or unclassified failure | `internal_error` |
| 2 | Invalid command, flag, or input | `usage_error` |
| 3 | Authentication is required or expired | `authentication_required` |
| 4 | The authenticated identity lacks permission | `forbidden` |
| 5 | The requested resource does not exist | `not_found` |
| 6 | The request conflicts with current state | `conflict` |
| 7 | The API or a required service is unavailable | `unavailable` |

Minor releases may add fields to `data`, `error.details`, commands, and flags. Do not rename existing envelope fields or change their meaning. Consumers must ignore unknown JSON fields.

Human output supports English and Spanish. `--language en|es` and `DAIKU_LANG` select the language. Without either setting, the CLI checks `LC_ALL`, `LC_MESSAGES`, and then `LANG`. The `C` and `POSIX` locales select English. Commands, flags, JSON fields, and machine error codes are never translated. `NO_COLOR` disables ANSI styling.

Destructive commands such as `profile remove` and `auth logout` require the full localized confirmation word in an interactive terminal. Non-interactive callers and destructive JSON calls must pass `--yes`. The CLI never reads confirmation from a pipe.

## Preserve authentication boundaries

OAuth login uses Authorization Code with Proof Key for Code Exchange (PKCE) S256 and an IPv4 loopback callback on a random dynamic port. If the CLI cannot open a browser, interactive mode prints the authorization URL and continues waiting for the callback. Agent mode and other non-interactive callers cannot complete login.

Credentials use the operating-system keychain by default: Keychain on macOS and Secret Service on Linux. A keychain error is fatal and must never trigger a silent fallback. Headless systems may explicitly set `DAIKU_CREDENTIAL_STORE=file` after accepting the host access-control and file-permission trust model. Credential files use atomic replacement and mode `0600`. Environment variables cannot provide OAuth tokens.

`auth logout` revokes the refresh token before deleting its local copy. If Daiku is unavailable, logout preserves the local credential so revocation can be retried. `--local-only` is the explicit escape hatch.

## Update the API contract

The typed client in `generated/daikuv1` is generated from `openapi/daiku-v1.json`. Do not edit generated files by hand.

`openapi/SOURCE.json` records the exact Daiku repository commit and export command. The SHA-256 file and `METHOD PATH operationId` manifest make contract changes explicit during review.

After exporting a new schema from the recorded Daiku commit, update the checksum and operation manifest, then run:

```sh
make contract-generate
make contract-check
```

`contract-check` regenerates into a temporary directory and compares the result byte for byte. It rejects checksum drift, broken schema references, stale generated files, missing or duplicate operation IDs, and unreviewed operation changes. The generator is pinned as a Go tool in `go.mod`.

## Maintain agent integrations

The portable Agent Skill in `skills/daiku` teaches Codex and Claude how to compose safe workflows from the live command contract. Refresh its checked-in command manifest and compact reference after changing commands or flags:

```sh
make skill-generate
make skill-check
```

The Codex and Claude installers update only the Daiku skill, refuse to replace an unrelated skill, and perform atomic updates. Preserve those invariants when changing `scripts/skill/`.

The Model Context Protocol (MCP) server derives one typed tool per runnable leaf command from the Cobra tree. Positional values use the `arguments` array. Flags become typed properties with underscores instead of hyphens. Required Cobra flags remain required in the tool schema.

MCP calls run existing command handlers in agent mode and use the same profile, credential store, validation, and API clients as the CLI. The server must remain read-only unless its owner passes `--allow-write`; every write call must also pass `confirm: true`. Protocol messages are the only stdout output. Diagnostics go to stderr, and results must redact secret-bearing fields.

## Keep diagnostics safe

`daiku doctor` is a read-only, deterministic diagnostic. It checks installation, profile and credential shape, safe token metadata, network reachability, schema metadata, installed Agent Skills, MCP readiness, and the OAuth callback.

Doctor must never refresh or write credentials, send an expired token, or expose token values. Warnings and failed health checks are findings rather than command failures, so a completed report exits 0. Invalid usage exits 2. Construction or rendering failures exit 1. Cancellation exits 6 without a partial report.

## Use worktrees and stacked pull requests

Implement each card in its own Git worktree and branch. Never run two agents in the same checkout. Branches use `feat/fizzy-card-slug` or `fix/fizzy-card-slug` and identify their Fizzy card in the pull request.

Keep stacks narrow and dependency ordered:

1. Branch the first card from `main`.
2. Branch dependent cards from the previous stack branch.
3. Open one focused pull request per card and state its base and dependency.
4. Rebase the remaining stack after a human merges its base.
5. Never merge, force-push, or bypass checks on an agent's behalf.

Parallel cards should own separate command-domain packages. If two cards need the executable composition root, sequence that small integration change in stack order.

## Hand off a pull request

Include the Fizzy card, stack position, API schema commit when relevant, tests run, risk, and rollback notes. Disclose substantial AI-assisted implementation or documentation work. A separate agent reviews the diff, and a human performs the merge.
