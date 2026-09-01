---
name: daiku
description: Read and manage personal-finance data through the authenticated Daiku CLI. Use for Daiku households, accounts, transactions, transfers, installments, recurring entries, budgets, portfolios, reports, or projections. Do not use for generic financial advice that does not require Daiku data or actions.
---

# Daiku

Turn the user's request into the smallest safe set of Daiku CLI calls. Preserve financial meaning, operate within the resolved profile and household, and report only outcomes confirmed by Daiku.

## Establish context

Run operational commands with `--agent`. It enables stable JSON, disables prompts and terminal styling, and returns breadcrumbs. Read stdout only on exit 0; read the JSON error from stderr otherwise. Preserve the exit code and ignore unknown response fields.

When Daiku MCP tools are available, use the typed tool corresponding to the CLI path instead of spawning the CLI. Apply the same discovery and safety rules. Write tools may be absent unless the MCP owner enabled them, and each write needs `confirm: true` after the user's intent authorizes the resolved mutation.

Check `daiku auth status --agent` before authenticated work when session state is unknown. OAuth login requires a person and cannot run in agent mode.

For household-scoped work, run `daiku households current --agent` when the selected household is unknown. A selected household supplies the default for every `--household` flag. If none is selected, use `households list` and pass the resolved ID explicitly. Do not change the selected household merely to complete a one-off request. Use `households use <name-or-id>` only when the user asks to set or switch the default.

Use the checked-in [command reference](references/commands.md) for routing. When syntax, required values, or available enums may differ in the installed version, run focused help such as `daiku help transactions create --agent`. Use `daiku commands --agent` only when the command family itself is unclear; live metadata wins over checked-in references.

## Resolve before acting

Resolve names to identifiers with read-only list/get commands. Never invent an identifier. For account operations, verify that the account belongs to the resolved household and note its currency. For portfolio operations, distinguish a named portfolio from the user-wide aggregate.

Convert relative dates such as “yesterday” into explicit `YYYY-MM-DD` values in the user's timezone. Ask when the timezone or intended accounting date is genuinely ambiguous. Preserve decimal strings as supplied; do not round, change signs, substitute currencies, or infer an exchange rate.

If multiple resources match, stop and ask one concise question that names the competing choices. If only an optional classification such as category or tag is missing, omit it rather than inventing it.

## Preserve finance semantics

- Create expenses and income with `transactions create` and the explicit `--type`. `--amount` uses the transaction currency; `--account-amount` is the amount posted in the selected account's currency.
- Create balanced movements with `transfers create`, never as unrelated expense and income records. Preserve source and destination direction. Use `--to-amount` when the destination received a different numeric amount.
- Use `accounts adjust` to reconcile an account to a target balance. Do not synthesize a normal transaction when the user requested a balance adjustment.
- For installments, `--amount` is the purchase total, never one installment. For recurring activity, create a recurring template instead of pre-creating future transactions.
- Treat budget summaries, portfolio totals, reports, exchange rates, and projections as server-calculated. Do not replace them with arithmetic over partial lists.
- Do not assume a fixed currency set. Use the currencies and enums published by focused help or the server contract.

Read [references/workflows.md](references/workflows.md) before a write, bulk operation, installment, recurring workflow, balance adjustment, or projection-rule change. It contains the domain-specific preflight and verification steps.

## Apply mutation boundaries

Read-only requests may run after scope is resolved. An ordinary reversible create or update may run when the user clearly requested it and every material value is known.

Before any delete, archive, unlink, bulk update/delete, balance adjustment, transfer conversion, profile removal, logout, or other command that exposes `--yes`, read [references/safety.md](references/safety.md). Do not add `--yes` to bypass a CLI rejection. MCP's `confirm: true` is an execution gate, not evidence of user authorization.

A timeout or transport failure during a mutation leaves the outcome unknown. Inspect the narrowest relevant state before considering a retry. Never retry a financial write blindly.

## Report the result

After a write, verify the returned object or fetch it once when the immediate response does not prove the persisted state. Check the type, account, amount, currency, date, and relationship fields that matter to the request.

Summarize the household or portfolio scope, action, important amounts and currencies, and persisted result. If Daiku rejects the request, report the stable error and a safe next action. Never claim success from intent, a request payload, or exit 0 when the returned object contradicts the requested financial meaning.

## References

- [workflows.md](references/workflows.md): domain preflight, execution, and verification patterns
- [safety.md](references/safety.md): authorization, destructive operations, retries, and error recovery
- [commands.md](references/commands.md): generated compact command reference when live help is unavailable
- [commands.json](references/commands.json): generated machine-readable command manifest; do not edit it by hand
