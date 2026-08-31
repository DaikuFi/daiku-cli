---
name: daiku
description: Manage personal finances through the Daiku CLI, including transactions, transfers, budgets, accounts, portfolios, reports, and projections. Use when a request needs Daiku data or a Daiku mutation; do not use for generic financial advice without Daiku.
---

# Daiku

Translate the user's intent into Daiku CLI calls while keeping identifiers, amounts, currencies,
and authorization boundaries explicit.

## Discover before composing

Use `daiku commands --agent` as the live source of truth. For a focused command, use
`daiku help <path> --agent`. The generated [command reference](references/commands.md) is useful for
routing when live introspection is unavailable, but live metadata wins if they differ.

Run operational commands with `--agent`. It guarantees the stable JSON envelope, disables input
and styling, and includes breadcrumbs. Read stdout on success, stderr on failure, and preserve the
process exit code. Ignore unknown response fields.

Before an authenticated workflow, establish the active profile and discover required resource IDs
with read-only list/get commands. Never invent IDs or infer an account, household, currency, date,
transfer direction, or amount when more than one reasonable interpretation exists.

## Compose finance operations

- Expenses and income are transactions. Preserve the user's decimal amount and ISO currency; use
  catalog/list commands to resolve household, account, category, and tag IDs.
- Transfers are balanced movements, not an expense plus an income. Resolve source and destination
  accounts and use `transfers create`; include `--to-amount` when the legs use different amounts.
- Treat budgets, balances, portfolio totals, net worth, and reports as server-calculated. Query the
  relevant summary/report command rather than recomputing authoritative totals from partial lists.
- Prefer the narrowest read needed. Continue through response breadcrumbs or structured help when
  the next command or required flag is unclear.

Summarize what Daiku returned without silently changing units, signs, dates, or currencies. If the
API reports an error, explain the safe message and remediation; do not claim the operation worked.

## Mutations and ambiguity

Read-only requests may run once their scope is resolved. Ordinary, reversible creates or updates
may run when the user clearly requested the change and every required value is known.

Do not execute destructive or structurally disruptive operations—including delete, unlink,
logout, profile removal, or transfer conversion—unless the user explicitly requested that exact
mutation and confirmed the resolved targets. Do not add `--yes` merely to overcome a rejection.
If intent or targets are ambiguous, stop before mutation and ask one concise question naming the
missing choice. A failed or timed-out mutation has unknown outcome: inspect state before any retry.

## Reference

Read [references/commands.md](references/commands.md) when live introspection cannot run. The
machine-readable [command manifest](references/commands.json) is generated from the same command
tree for tooling; do not hand-edit either file.
