# Daiku mutation safety

Use this reference for commands that can delete data, alter balances or relationships, affect many records, or mutate local authentication and scope.

## Separate authorization from execution gates

The user's request authorizes only the described action and targets. A CLI `--yes` flag or MCP `confirm: true` allows execution after authorization; neither creates authorization.

Before a gated command:

1. Resolve every target with read-only commands.
2. State the active household or portfolio and the exact resource identifiers.
3. State the financial effect, including amounts, currencies, direction, scope, and affected record count when available.
4. Obtain explicit confirmation for that resolved action unless the user already gave it with the same specificity in the current request.
5. Execute once with the required gate.

Never broaden “clean this up,” “fix it,” or similar language into deletion, unlinking, conversion, bulk mutation, logout, or profile removal.

## Commands requiring destructive preflight

Apply the procedure to:

- Transaction, recurring template, projection rule/scenario, portfolio, asset, bucket, account, household, category, tag, and institution deletion or archival
- `transactions delete-all`, bulk update, and any bulk delete
- `transfers convert` and `transfers unlink`
- `accounts adjust`
- Installment updates whose scope affects future transactions or the entire plan
- Profile removal and authentication logout
- Changing or clearing the selected profile or household when that change was not the user's primary request

Treat a command as gated whenever focused help exposes `--yes`, even if it is not listed here.

## Avoid collateral changes

- Do not change the active profile or selected household to make a one-off command shorter. Pass an explicit scope instead.
- Do not mutate categories, tags, accounts, or portfolios merely because a requested transaction references a missing optional classification.
- Do not convert a transaction to a transfer when creating a new balanced transfer satisfies the request.
- Do not delete and recreate a resource to work around an update error.
- Do not use `delete-all` when individual resolved deletions meet the user's scope.

## Handle failures without duplicate writes

Classify the result by exit code and JSON error code:

- Exit 2: fix local usage or missing values; no server mutation should have occurred.
- Exit 3: a person must restore authentication when login is required.
- Exit 4: stop. The identity or credential type lacks permission.
- Exit 5: re-check the resource ID and active scope. Do not create a replacement automatically.
- Exit 6: inspect current state before changing the request.
- Exit 7 or a timeout: the write outcome may be unknown.

After an ambiguous transport failure, query by the narrowest available identifiers and distinctive fields such as account, amount, currency, date, and description. Retry only when the read proves the first write did not persist. If the state cannot prove that, report the uncertainty and stop.

## Verify financial invariants

After mutation, verify the invariants relevant to the operation:

- Transaction: type, income/expense semantics, account, amount, account amount, currency, date, category, and tags
- Transfer: two legs, peer relationship, distinct accounts, direction, source amount, destination amount, and restored balances when deleted
- Balance adjustment: exact target balance and one adjustment record
- Installments: purchase total, count, schedule, plan scope, and materialized rows
- Recurring: template type, frequency, mode, accounts, next occurrence, and occurrence status
- Portfolio asset: native value, liability status, valuation currency, bucket, and totals
- Projection: scenario, rules, starting snapshot, and absence of unexplained jumps

An exit-0 response that contradicts a requested invariant is not success. Report the mismatch, preserve the returned identifiers for diagnosis, and do not attempt a compensating write without authorization.
