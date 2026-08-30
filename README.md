# Daiku CLI

`daiku` is the command-line client for [Daiku](https://daiku.app). It is designed for people at a terminal and for agents composing reliable workflows.

The CLI includes a modular Cobra runtime, deterministic machine output, stable exit codes, named profiles, and OAuth authentication for macOS and Linux.

## Authentication and profiles

Create a profile, then sign in through the browser:

```sh
daiku profile add personal
daiku auth login
daiku auth status
```

Use `daiku profile list`, `daiku profile use <name>`, and `daiku profile remove <name>` to manage isolated identities. `auth login` uses Authorization Code with PKCE S256 and an IPv4 loopback callback on a random dynamic port. If the browser cannot be launched, interactive mode prints a URL and keeps waiting for the callback.

Credentials use the operating-system keychain by default (Keychain on macOS and Secret Service on Linux). A keychain error is fatal and never causes a silent downgrade. Headless systems may explicitly accept a host-access-control and file-permission trust model with `DAIKU_CREDENTIAL_STORE=file`; those credential files are atomically replaced with mode `0600`. Environment variables cannot supply OAuth tokens.

`daiku auth logout` revokes the refresh token before deleting its local copy. If Daiku is unavailable, it preserves the local credential so revocation can be retried; `--local-only` is the explicit escape hatch.

## Install for development

You need Go 1.24 or newer.

```sh
git clone git@github.com:DaikuFi/daiku-cli.git
cd daiku-cli
make build
./bin/daiku --help
```

## Output contract

Interactive output is concise and may use ANSI styling when stdout is a terminal. Redirected output never includes ANSI escapes. Scripts and agents should always pass `--json`; JSON field names and commands are in English.

Human output supports English and Spanish. Select it explicitly with `--language en|es` or set
`DAIKU_LANG`; otherwise the CLI consults `LC_ALL`, `LC_MESSAGES`, then `LANG`. `C` and `POSIX` use
English. Set `NO_COLOR` to disable ANSI styling. These settings never translate commands, flags,
JSON fields, or machine error codes.

Destructive commands such as `profile remove` and `auth logout` require the full localized
confirmation word in an interactive terminal. Non-interactive callers and destructive `--json` calls must
pass `--yes`; the CLI never reads a confirmation from a pipe.

Successful JSON output uses one envelope:

```json
{"ok":true,"data":{"version":"0.1.0"}}
```

Errors are written to stderr and use one envelope:

```json
{"ok":false,"error":{"code":"usage_error","message":"unknown command \"example\" for \"daiku\""}}
```

The process exit code remains authoritative even when JSON output is enabled:

| Exit | Meaning | Stable error category |
| ---: | --- | --- |
| 0 | Success | — |
| 1 | Internal or unclassified failure | `internal_error` |
| 2 | Invalid command, flag, or input | `usage_error` |
| 3 | Authentication is required or expired | `authentication_required` |
| 4 | The authenticated identity lacks permission | `forbidden` |
| 5 | The requested resource does not exist | `not_found` |
| 6 | The request conflicts with current state | `conflict` |
| 7 | The API or a required service is unavailable | `unavailable` |

Minor releases may add fields to `data`, `error.details`, commands, and flags. They will not rename or change the meaning of existing envelope fields or exit codes. Agents should ignore unknown JSON fields.

## Command architecture

Each command domain implements `cli.Module` in its own package and owns its Cobra subtree. The executable explicitly injects modules into the app; packages must not use `init()` or mutate a global command registry. This keeps construction testable and limits overlap between parallel feature branches.

```go
app := cli.New(
    cli.WithModule(versioncommand.New(version)),
    cli.WithModule(accountscommand.New(client)),
)
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for checks and the stacked-PR workflow.
