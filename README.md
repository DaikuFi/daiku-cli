# Daiku CLI

Use Daiku from your terminal or connect it to an agent. The CLI covers daily finance work, including transactions, transfers, budgets, accounts, portfolios, reports, and projections.

Commands work in English. Human output supports English and Spanish. Scripts and agents receive stable JSON.

## Install Daiku

Install the latest stable release with Homebrew:

```sh
brew install DaikuFi/tap/daiku
```

Confirm that your shell finds the installed binary:

```sh
daiku version
daiku doctor
```

Homebrew installs signed releases for macOS on Apple Silicon and Intel. The project also publishes Linux archives for amd64 and arm64.

Linux users can download the archive for their platform from [GitHub Releases](https://github.com/DaikuFi/daiku-cli/releases). Follow the [signed artifact instructions](RELEASE.md#verify-signed-artifacts) to verify its checksum and provenance.

## Sign in

Create a profile, complete OAuth in your browser, and check the session:

```sh
daiku profile add personal
daiku auth login
daiku auth status
```

A profile keeps one Daiku server and its credentials separate from your other environments. Use `daiku profile list` and `daiku profile use profile_name` when you have more than one.

The CLI stores credentials in Keychain on macOS and Secret Service on Linux. OAuth login requires a browser and a local callback, so a person must complete it before an agent can use the authenticated CLI.

List your households to confirm that the connection works:

```sh
daiku households list
```

## Run common finance commands

Daiku resources use identifiers (IDs). Start with a list command, copy the ID you need, and pass it to the next command. The IDs below are recognizable placeholders.

List accounts and recent transactions:

```sh
daiku accounts list --household hsh_1234567890123
daiku transactions list \
  --household hsh_1234567890123 \
  --month 9 \
  --year 2026 \
  --all
```

Search and sort transactions:

```sh
daiku transactions list \
  --household hsh_1234567890123 \
  --query supermercado \
  --ordering amount_high \
  --all
```

Check a monthly budget in a chosen currency:

```sh
daiku budgets summary \
  --household hsh_1234567890123 \
  --month 9 \
  --year 2026 \
  --currency UYU
```

Inspect a portfolio and its net worth history:

```sh
daiku portfolios totals pfl_1234567890123
daiku reports net-worth \
  --portfolio pfl_1234567890123 \
  --currency USD \
  --months 12
```

Run `daiku help command_name` for flags and examples available in your installed version:

```sh
daiku help transactions create
daiku help transfers create
daiku help projections calculate
```

Set `--language es` or `DAIKU_LANG=es` for Spanish human output. Commands, flags, and JSON fields stay in English.

## Use Daiku from an agent

Daiku supports agents through a portable Agent Skill, deterministic agent mode, and a Model Context Protocol (MCP) server. Authenticate once as a person before using any of them.

### Install the Agent Skill

The Agent Skill teaches Codex or Claude how to discover IDs, choose the correct command, preserve currencies and signs, and stop before ambiguous or destructive changes.

Clone the repository, then install the skill for the agent you use:

```sh
git clone --depth 1 https://github.com/DaikuFi/daiku-cli.git
cd daiku-cli
./scripts/skill/install-codex.sh
```

For Claude, run:

```sh
./scripts/skill/install-claude.sh
```

The installers update only the Daiku skill. They refuse to replace an unrelated skill at the same path.

### Connect through MCP

Point any stdio-compatible MCP client at this command:

```sh
daiku mcp
```

The MCP server exposes one typed tool per runnable CLI command. It is read-only by default. Start it with `daiku mcp --allow-write` only when you want write tools available. Each write call must also include `confirm: true`.

### Use agent mode directly

Agents that run shell commands should pass `--agent`:

```sh
daiku households list --agent
daiku commands --agent
daiku help transactions create --agent
```

Agent mode implies JSON output, disables interactive input and terminal styling, and includes breadcrumbs for the next safe action. Use `--json` instead when a script needs JSON but may still handle interaction itself.

## Write better prompts

You do not need to find Daiku IDs before asking an agent. Name the household, account, date, amount, and currency when they matter. The agent can resolve names to IDs with read-only commands.

Add `Solo lectura` when you want analysis without changes. For a write, ask the agent to show the resolved resources before it runs the command. State the output you want, such as a table, totals by category, or a short list of anomalies.

Prompts can be written in Spanish or English. These examples use Spanish because Daiku primarily serves users in Uruguay.

### Review spending

```text
En el household Mi Casa, mostrame todos los gastos de agosto de 2026,
agrupalos por categoría y comparalos con el presupuesto del mes.
Usá UYU para los totales. Solo lectura.
```

### Find possible duplicates

```text
Buscá transacciones posiblemente duplicadas en Mi Casa durante los últimos
30 días. Compará fecha, cuenta, moneda, importe y descripción. No borres ni
modifiques nada. Devolveme una tabla con las coincidencias más probables.
```

### Prepare a transaction

```text
Prepará un ingreso de 85.000 UYU con fecha 2026-09-01 en la cuenta Sueldo,
descripción "Salario septiembre". Antes de crearlo, mostrame el household,
la cuenta y los importes que resolviste. Esperá mi confirmación.
```

### Record a transfer

```text
Transferí 200 USD desde Itaú USD hacia Ahorro USD en Mi Casa con fecha
2026-09-01. Antes de escribir, confirmá las dos cuentas y que no vas a
registrarlo como gasto e ingreso separados.
```

### Check net worth and currency exposure

```text
Mostrame el patrimonio neto de los últimos 12 meses y la exposición por
moneda. Separá el portfolio Inversiones del total general y marcá cambios
que superen 10%. Solo lectura.
```

### Compare projection scenarios

```text
Compará los escenarios Base y Retiro temprano del portfolio FIRE. Explicá
la diferencia en fecha de retiro, activos finales y supuestos. No cambies
reglas ni escenarios.
```

## Inspect available commands

The CLI is the source of truth for its installed command set:

```sh
daiku --help
daiku commands --json
daiku help transactions create --agent
```

The generated [command reference](skills/daiku/references/commands.md) is available when live introspection cannot run.

Daiku currently covers:

- Households, account groups, accounts, institutions, categories, and tags
- Transactions, transfers, installments, and recurring entries
- Budget rules, planned budgets, summaries, and exchange rates
- Portfolios, buckets, assets, cash flows, and value history
- Net worth, currency exposure, projection scenarios, and projection rules

## Diagnose problems

Run the read-only diagnostic before changing configuration:

```sh
daiku doctor
```

Use these focused checks when login or command discovery fails:

```sh
daiku auth status --json
daiku profile list
daiku commands --agent
```

If the access token expired, run `daiku auth login` again. If `doctor` reports a missing Agent Skill, reinstall it from a current repository checkout.

## Contribute

Read [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, architecture, machine-output contracts, API generation, Agent Skill maintenance, and pull request conventions. Maintainers should follow [RELEASE.md](RELEASE.md) for signed candidates and Homebrew publication.

## License

Daiku CLI is licensed under the [Apache License, Version 2.0](LICENSE). See [NOTICE](NOTICE) for attribution requirements.
