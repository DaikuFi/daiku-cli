# Daiku CLI

`daiku` is the command-line client for [Daiku](https://daiku.app). It is designed for people at a terminal and for agents composing reliable workflows.

This repository currently contains the CLI foundation: a modular Cobra runtime, deterministic machine output, stable exit codes, tests, and macOS/Linux builds. Domain commands and OAuth authentication will arrive in subsequent stacked pull requests.

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
