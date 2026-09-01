# Daiku workflow patterns

Read only the section that matches the user's task. Use focused live help before execution when the installed CLI contract may differ.

## Select scope without changing it

1. Check the active session with `daiku auth status --agent` when needed.
2. Check `daiku households current --agent` for household-scoped work.
3. If a household is selected and matches the request, omit `--household`.
4. If the request names another household, resolve it with `households list` and pass its ID for that command. Do not switch the persisted default.
5. Run `households use <name-or-id> --agent` only when the user asks to change the default. The selection belongs to the active profile.

Portfolio commands are user-scoped rather than household-scoped. Resolve the portfolio independently. Reports without `--portfolio` use the user-wide aggregate; do not describe that result as one portfolio.

## Read transactions and analyze spending

- Use `transactions list` for structured filters and `transactions search` for a required text query.
- Use explicit month/year or from/to dates. Do not silently mix periods.
- Use `--all` only when the requested analysis needs the complete matching set. Otherwise preserve pagination metadata and state that the result is partial.
- `--type transfer` is valid for transfer filtering. Omitting type also includes adjustments.
- Keep native currencies visible when grouping. Convert totals only through a server-calculated report, budget response, or exchange rate appropriate to the transaction date.
- Prefer `budgets summary`, `budgets planned`, portfolio totals, and reports for authoritative totals. Transaction aggregation is appropriate for ad hoc breakdowns, not as a replacement for those endpoints.

## Create or update a transaction

Preflight:

1. Resolve the household and account. Record the account currency.
2. Resolve category and tag identifiers only when the user supplied or clearly implied them.
3. Resolve the accounting date explicitly.
4. Preserve `expense` versus `income`; do not infer the type from a positive number.
5. Treat `--amount` as the amount in `--currency`. Use `--account-amount` only when the amount posted to the account differs, commonly across currencies.

Execute one `transactions create` or `transactions update` call. Do not add `--yes`; these commands do not need it.

Verify the persisted transaction type, `is_income` when returned, account, amount, account amount, currency, date, description, category, and tags. If an update reports success but a requested field did not change, report the mismatch as a failure.

For bulk create or update, inspect focused help and the input schema first. Validate the entire file before execution. Bulk update changes many records and requires the safety procedure.

## Create or modify a transfer

Preflight:

1. Resolve distinct source and destination accounts in the same household.
2. Confirm direction from the user's wording.
3. Use `--amount` for the source-account amount.
4. Use `--to-amount` for the destination-account amount when it differs. Never calculate a replacement amount or rate unless the user explicitly asked for that calculation and supplied the necessary basis.

Use `transfers create` for a new transfer. Use `transfers convert` only when the user explicitly asks to convert an existing transaction, and follow the destructive-operation safety procedure. Use `transfers unlink` only when the user explicitly wants both transfer legs separated.

Verify both legs, their transfer relationship, currencies, amounts, accounts, date, and direction. If the create response omits relationship fields, list or get the resulting transactions before reporting success.

## Reconcile an account balance

`accounts adjust --target-balance` sets the account to a target balance; it is not a delta. Resolve the account, read its current balance, and show the current and target balances before asking for confirmation. After execution, fetch the account and confirm the target balance exactly.

Do not create an expense or income as a substitute. Do not adjust a balance when the user only asked why two totals differ.

## Create installments

- `--amount` is the full purchase total, never the amount of one installment.
- `--count` is the total installment count.
- Resolve the card or account, category, tags, purchase date, currency, and account amount.
- Expect Daiku to materialize the appropriate schedule. Do not create the future transactions yourself.
- When updating, distinguish `this`, `future`, and `plan` scope using focused help. Ask if the desired scope is unclear.

Verify the plan total, count, schedule, materialized transaction count, and selected account.

## Create recurring activity

Resolve `expense`, `income`, or `transfer`; frequency; creation mode; date; and accounts. A recurring transfer needs both source and destination accounts. A yearly template needs its month. `auto` and `confirm` have materially different behavior, so never choose between them silently.

Use occurrence commands for generated occurrences:

- `confirm` creates the real human entry.
- `skip` dismisses one pending occurrence.
- `snooze` changes its reminder date.

Verify the template or occurrence state after mutation. Do not pre-create a series of normal transactions.

## Work with budgets

- Use `budgets summary` for actual versus planned values in one month.
- Use `budgets planned` for the server-calculated year plan.
- Use budget rule commands to inspect or change category rules.
- Use `budgets suggestions` only when the authenticated credential type supports it. A typed forbidden response is not permission to substitute a different endpoint or token.

Preserve the response currency and period. Do not recompute remaining budget from a partial transaction page.

## Work with portfolios and reports

Resolve the portfolio for holdings, totals, assets, buckets, cash flows, and value history. Use `portfolios totals` for the authoritative current total.

For reports:

- Pass `--portfolio` for a single portfolio.
- Omit it only when the user requests the user-wide aggregate.
- Preserve the report currency, snapshot date, and date range.
- Use `reports currency-exposure` for exposure rather than grouping asset rows manually.
- Use `reports net-worth` for the historical series rather than reconstructing snapshots.

## Work with projections

Resolve the portfolio and scenario independently. Before creating or updating a projection rule:

1. Run `projections rule-types list`.
2. Fetch the chosen type with `projections rule-types get <type>`.
3. Build `--config` only from the returned `config_fields` contract.
4. Resolve referenced bucket, asset, currency, frequency, and category values.
5. Show the proposed rule and assumptions before mutation.

After changing a rule, fetch the rule and calculate the scenario. Compare month 0 with current portfolio totals and call out unexplained discontinuities instead of presenting them as valid forecasts.
