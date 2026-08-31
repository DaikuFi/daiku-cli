# Daiku command reference

Generated from `daiku commands --agent`; do not edit by hand. Live introspection wins if this reference differs.

## `daiku account-groups create`

Create account group

Usage: `daiku account-groups create`

Flags:

- `--emoji` (string): emoji
- `--household` (string) (required): household ID
- `--name` (string) (required): name

## `daiku account-groups delete`

Delete account group

Usage: `daiku account-groups delete <resource>`

Flags:

- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku account-groups list`

List account groups

Usage: `daiku account-groups list`

Flags:

- `--household` (string) (required): household ID

## `daiku account-groups reorder`

Reorder account groups

Usage: `daiku account-groups reorder`

Flags:

- `--household` (string) (required): household ID
- `--id` (stringSlice): resource IDs in desired order

## `daiku account-groups update`

Update account group

Usage: `daiku account-groups update <resource>`

Flags:

- `--emoji` (string): emoji
- `--household` (string) (required): household ID
- `--name` (string): name

## `daiku accounts adjust`

Adjust an account balance

Usage: `daiku accounts adjust <account>`

Flags:

- `--date` (string): adjustment date
- `--household` (string) (required): household ID
- `--note` (string): adjustment note
- `--target-balance` (string) (required): target balance
- `--yes` (bool): skip the interactive confirmation

## `daiku accounts archive`

Archive an account

Usage: `daiku accounts archive <account>`

Flags:

- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku accounts create`

Create an account

Usage: `daiku accounts create`

Flags:

- `--account-holder` (string): account holder
- `--account-number` (string): account number
- `--currency` (string): currency
- `--emoji` (string): emoji
- `--group` (string): account group ID
- `--household` (string) (required): household ID
- `--institution` (string): institution ID
- `--is-default` (bool): make this the default account
- `--name` (string) (required): account name
- `--opening-balance` (string): opening balance
- `--type` (string): account type: checking, savings, credit_card, loan, investment, cash, other

## `daiku accounts list`

List accounts

Usage: `daiku accounts list`

Flags:

- `--archived` (string): include archived accounts: true or all
- `--household` (string) (required): household ID

## `daiku accounts reorder`

Reorder accounts

Usage: `daiku accounts reorder`

Flags:

- `--household` (string) (required): household ID
- `--id` (stringSlice): resource IDs in desired order

## `daiku accounts unarchive`

Unarchive an account

Usage: `daiku accounts unarchive <account>`

Flags:

- `--household` (string) (required): household ID

## `daiku accounts update`

Update an account

Usage: `daiku accounts update <account>`

Flags:

- `--account-holder` (string): account holder
- `--account-number` (string): account number
- `--clear-group` (bool): clear the account group
- `--clear-institution` (bool): clear the institution
- `--currency` (string): currency
- `--emoji` (string): emoji
- `--group` (string): account group ID
- `--household` (string) (required): household ID
- `--institution` (string): institution ID
- `--is-default` (bool): make this the default account
- `--name` (string): account name
- `--opening-balance` (string): opening balance
- `--type` (string): account type: checking, savings, credit_card, loan, investment, cash, other

## `daiku assets cashflows create`

Create a cashflow

Usage: `daiku assets cashflows create`

Flags:

- `--asset` (string) (required): asset ID
- `--cash-in` (string): cash in amount
- `--cash-in-currency` (string): cash in currency
- `--cash-out` (string): cash out amount
- `--cash-out-currency` (string): cash out currency
- `--date` (string) (required): date (YYYY-MM-DD)
- `--notes` (string): notes

## `daiku assets cashflows delete`

Delete a cashflow

Usage: `daiku assets cashflows delete <id>`

Flags:

- `--asset` (string) (required): asset ID
- `--yes` (bool): skip the interactive confirmation

## `daiku assets cashflows list`

List cashflows, including transaction links

Usage: `daiku assets cashflows list`

Flags:

- `--asset` (string) (required): asset ID

## `daiku assets cashflows update`

Update a cashflow

Usage: `daiku assets cashflows update <id>`

Flags:

- `--asset` (string) (required): asset ID
- `--cash-in` (string): cash in amount
- `--cash-in-currency` (string): cash in currency
- `--cash-out` (string): cash out amount
- `--cash-out-currency` (string): cash out currency
- `--clear-cash-in` (bool): clear cash in amount
- `--clear-cash-in-currency` (bool): clear cash in currency
- `--clear-cash-out` (bool): clear cash out amount
- `--clear-cash-out-currency` (bool): clear cash out currency
- `--date` (string): date (YYYY-MM-DD)
- `--notes` (string): notes

## `daiku assets create`

Create an asset

Usage: `daiku assets create`

Flags:

- `--bucket` (string) (required): bucket ID
- `--currency` (string): ISO currency supported by Daiku
- `--current-value` (string): current value (sent verbatim to Daiku)
- `--exclude-from-projections` (bool): exclude from projections
- `--institution` (string): institution ID
- `--last-price-update` (string): last price update (RFC3339)
- `--liability` (bool): mark as liability
- `--name` (string) (required): asset name
- `--notes` (string): notes
- `--price-per-unit` (string): price per unit
- `--quantity` (string): quantity
- `--ticker` (string): ticker symbol
- `--type` (string) (required): asset type: checking, savings, brokerage, stock, etf, mutual_fund, bond, crypto_wallet, crypto_exchange, property, vehicle, loan, mortgage, credit_card, other

## `daiku assets delete`

Delete an asset

Usage: `daiku assets delete <id>`

Flags:

- `--bucket` (string) (required): bucket ID
- `--yes` (bool): skip the interactive confirmation

## `daiku assets list`

List assets

Usage: `daiku assets list`

Flags:

- `--bucket` (string) (required): bucket ID

## `daiku assets update`

Update an asset

Usage: `daiku assets update <id>`

Flags:

- `--bucket` (string) (required): bucket ID
- `--clear-last-price-update` (bool): clear last price update
- `--clear-price-per-unit` (bool): clear price per unit
- `--clear-quantity` (bool): clear quantity
- `--clear-ticker` (bool): clear ticker symbol
- `--currency` (string): ISO currency supported by Daiku
- `--current-value` (string): current value (sent verbatim to Daiku)
- `--exclude-from-projections` (bool): exclude from projections
- `--institution` (string): institution ID
- `--last-price-update` (string): last price update (RFC3339)
- `--liability` (bool): mark as liability
- `--name` (string): asset name
- `--notes` (string): notes
- `--price-per-unit` (string): price per unit
- `--quantity` (string): quantity
- `--ticker` (string): ticker symbol
- `--type` (string): asset type: checking, savings, brokerage, stock, etf, mutual_fund, bond, crypto_wallet, crypto_exchange, property, vehicle, loan, mortgage, credit_card, other

## `daiku assets value-history create`

Create a value-history point

Usage: `daiku assets value-history create`

Flags:

- `--asset` (string) (required): asset ID
- `--currency` (string): ISO currency supported by Daiku
- `--date` (string) (required): date (YYYY-MM-DD)
- `--notes` (string): notes
- `--quantity` (string): recorded quantity
- `--value` (string): recorded value

## `daiku assets value-history delete`

Delete a value-history point

Usage: `daiku assets value-history delete <id>`

Flags:

- `--asset` (string) (required): asset ID
- `--yes` (bool): skip the interactive confirmation

## `daiku assets value-history list`

List value history

Usage: `daiku assets value-history list`

Flags:

- `--asset` (string) (required): asset ID

## `daiku assets value-history update`

Update a value-history point

Usage: `daiku assets value-history update <id>`

Flags:

- `--asset` (string) (required): asset ID
- `--clear-quantity` (bool): clear recorded quantity
- `--currency` (string): ISO currency supported by Daiku
- `--date` (string): date (YYYY-MM-DD)
- `--notes` (string): notes
- `--quantity` (string): recorded quantity
- `--value` (string): recorded value

## `daiku auth login`

Sign in using OAuth

Usage: `daiku auth login`

## `daiku auth logout`

Revoke and remove credentials

Usage: `daiku auth logout`

Flags:

- `--local-only` (bool): remove local credentials without revoking the token
- `--yes` (bool): skip the interactive confirmation

## `daiku auth status`

Show authentication status

Usage: `daiku auth status`

## `daiku budgets`

Inspect budget summaries and manage budget rules

Usage: `daiku budgets`

## `daiku budgets planned`

Show planned budgets for a year

Usage: `daiku budgets planned`

Flags:

- `--currency` (string): ISO currency supported by Daiku
- `--household` (string) (required): household ID
- `--year` (int): calendar year

## `daiku budgets rules create`

Create a category budget rule

Usage: `daiku budgets rules create`

Flags:

- `--amount` (string) (required): budget amount
- `--category` (string) (required): category ID
- `--currency` (string) (required): ISO currency supported by Daiku
- `--household` (string) (required): household ID
- `--month` (int): month (required for month scope)
- `--scope` (string) (required): scope: monthly, yearly or month
- `--year` (int): pinned year (required for month scope)

## `daiku budgets rules delete`

Delete a category budget rule

Usage: `daiku budgets rules delete <id>`

Flags:

- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku budgets rules list`

List category budget rules

Usage: `daiku budgets rules list`

Flags:

- `--household` (string) (required): household ID

## `daiku budgets rules update`

Update a category budget rule

Usage: `daiku budgets rules update <id>`

Flags:

- `--amount` (string): budget amount
- `--category` (string): category ID
- `--clear-month` (bool): clear the pinned month
- `--clear-year` (bool): clear the pinned year
- `--currency` (string): ISO currency supported by Daiku
- `--household` (string) (required): household ID
- `--month` (int): month (required for month scope)
- `--scope` (string): scope: monthly, yearly or month
- `--year` (int): pinned year (required for month scope)

## `daiku budgets suggestions`

Show budget suggestions for a month

Usage: `daiku budgets suggestions`

Flags:

- `--currency` (string): ISO currency supported by Daiku
- `--household` (string) (required): household ID
- `--month` (int): month (1-12)
- `--year` (int): calendar year

## `daiku budgets summary`

Show a monthly budget summary

Usage: `daiku budgets summary`

Flags:

- `--currency` (string): ISO currency supported by Daiku
- `--household` (string) (required): household ID
- `--month` (int): month (1-12)
- `--year` (int): calendar year

## `daiku categories create`

Create category

Usage: `daiku categories create`

Flags:

- `--emoji` (string): emoji
- `--household` (string) (required): household ID
- `--name` (string) (required): name
- `--parent` (string): parent category ID

## `daiku categories delete`

Delete category

Usage: `daiku categories delete <resource>`

Flags:

- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku categories list`

List categories

Usage: `daiku categories list`

Flags:

- `--household` (string) (required): household ID

## `daiku categories reorder`

Reorder categories

Usage: `daiku categories reorder`

Flags:

- `--household` (string) (required): household ID
- `--id` (stringSlice): resource IDs in desired order

## `daiku categories update`

Update category

Usage: `daiku categories update <resource>`

Flags:

- `--clear-parent` (bool): clear the parent category
- `--emoji` (string): emoji
- `--household` (string) (required): household ID
- `--name` (string): name
- `--parent` (string): parent category ID

## `daiku commands`

List machine-readable command metadata

Usage: `daiku commands`

## `daiku completion bash`

Generate the autocompletion script for bash

Usage: `daiku completion bash`

Flags:

- `--no-descriptions` (bool): disable completion descriptions

## `daiku completion fish`

Generate the autocompletion script for fish

Usage: `daiku completion fish`

Flags:

- `--no-descriptions` (bool): disable completion descriptions

## `daiku completion powershell`

Generate the autocompletion script for powershell

Usage: `daiku completion powershell`

Flags:

- `--no-descriptions` (bool): disable completion descriptions

## `daiku completion zsh`

Generate the autocompletion script for zsh

Usage: `daiku completion zsh`

Flags:

- `--no-descriptions` (bool): disable completion descriptions

## `daiku exchange-rates`

List server-resolved exchange rates

Usage: `daiku exchange-rates`

Flags:

- `--date` (string): requested date (YYYY-MM-DD; server resolves prior business day)

## `daiku help`

Help about any command

Usage: `daiku help [command]`

## `daiku households create`

Create a household

Usage: `daiku households create`

Flags:

- `--display-currency` (string): display currency
- `--emoji` (string): household emoji
- `--name` (string) (required): household name

## `daiku households delete`

Delete a household

Usage: `daiku households delete <household>`

Flags:

- `--yes` (bool): skip the interactive confirmation

## `daiku households get`

Get a household

Usage: `daiku households get <household>`

## `daiku households list`

List households

Usage: `daiku households list`

## `daiku households mode`

Set household mode

Usage: `daiku households mode <household>`

Flags:

- `--uses-accounts` (bool) (required): account mode (required)

## `daiku households reorder`

Reorder households

Usage: `daiku households reorder`

Flags:

- `--id` (stringSlice): household IDs in desired order

## `daiku households update`

Update a household

Usage: `daiku households update <household>`

Flags:

- `--display-currency` (string): display currency
- `--emoji` (string): household emoji
- `--name` (string): household name

## `daiku installments create`

Create an installment plan

Usage: `daiku installments create`

Flags:

- `--account` (string): account ID
- `--account-amount` (string): amount posted to the selected account
- `--amount` (string) (required): purchase total as decimal string
- `--category` (string): category ID
- `--count` (int) (required): number of installments
- `--currency` (string) (required): currency code published by the installment API contract
- `--date` (string) (required): purchase date
- `--description` (string) (required): description
- `--household` (string) (required): household ID
- `--tag-ids` (stringSlice): tag ID (repeatable)

## `daiku installments get`

Show an installment plan

Usage: `daiku installments get ID`

Flags:

- `--household` (string) (required): household ID

## `daiku installments list`

List installment plans

Usage: `daiku installments list`

Flags:

- `--household` (string) (required): household ID

## `daiku installments update`

Update an installment plan

Usage: `daiku installments update ID`

Flags:

- `--account` (string): account ID
- `--amount` (string): purchase total, never cuota amount
- `--category` (string): category ID
- `--clear-account` (bool): set account to null
- `--clear-category` (bool): set category to null
- `--currency` (string): currency code published by the installment API contract
- `--description` (string): description
- `--household` (string) (required): household ID

## `daiku institutions create`

Create institution

Usage: `daiku institutions create`

Flags:

- `--country` (string): ISO country code
- `--domain` (string): domain
- `--household` (string) (required): household ID
- `--name` (string) (required): name

## `daiku institutions delete`

Delete institution

Usage: `daiku institutions delete <resource>`

Flags:

- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku institutions list`

List institutions

Usage: `daiku institutions list`

Flags:

- `--household` (string) (required): household ID

## `daiku institutions update`

Update institution

Usage: `daiku institutions update <resource>`

Flags:

- `--country` (string): ISO country code
- `--domain` (string): domain
- `--household` (string) (required): household ID
- `--name` (string): name

## `daiku mcp`

Run the Daiku MCP server over stdio

Usage: `daiku mcp`

Flags:

- `--allow-write` (bool): allow confirmed write tools

## `daiku portfolios buckets create`

Create a bucket

Usage: `daiku portfolios buckets create`

Flags:

- `--emoji` (string): bucket emoji
- `--name` (string) (required): bucket name
- `--portfolio` (string) (required): portfolio ID
- `--sort-order` (int): sort order
- `--type` (string) (required): bucket type: cash, investments, crypto, real_estate, vehicles, other

## `daiku portfolios buckets delete`

Delete a bucket

Usage: `daiku portfolios buckets delete <id>`

Flags:

- `--portfolio` (string) (required): portfolio ID
- `--yes` (bool): skip the interactive confirmation

## `daiku portfolios buckets list`

List buckets

Usage: `daiku portfolios buckets list`

Flags:

- `--portfolio` (string) (required): portfolio ID

## `daiku portfolios buckets update`

Update a bucket

Usage: `daiku portfolios buckets update <id>`

Flags:

- `--emoji` (string): bucket emoji
- `--name` (string): bucket name
- `--portfolio` (string) (required): portfolio ID
- `--sort-order` (int): sort order
- `--type` (string): bucket type: cash, investments, crypto, real_estate, vehicles, other

## `daiku portfolios create`

Create a portfolio

Usage: `daiku portfolios create`

Flags:

- `--default` (bool): make this the default portfolio
- `--display-currency` (string): ISO currency supported by Daiku
- `--emoji` (string): portfolio emoji
- `--name` (string) (required): portfolio name

## `daiku portfolios delete`

Delete a portfolio

Usage: `daiku portfolios delete <id>`

Flags:

- `--yes` (bool): skip the interactive confirmation

## `daiku portfolios get`

Get a portfolio

Usage: `daiku portfolios get <id>`

## `daiku portfolios holdings`

Show server-calculated portfolio holdings

Usage: `daiku portfolios holdings <id>`

## `daiku portfolios list`

List portfolios

Usage: `daiku portfolios list`

## `daiku portfolios totals`

Show server-calculated assets, liabilities and net worth

Usage: `daiku portfolios totals <id>`

## `daiku portfolios update`

Update a portfolio

Usage: `daiku portfolios update <id>`

Flags:

- `--default` (bool): make this the default portfolio
- `--display-currency` (string): ISO currency supported by Daiku
- `--emoji` (string): portfolio emoji
- `--name` (string): portfolio name

## `daiku profile add`

Add a profile

Usage: `daiku profile add <name>`

Flags:

- `--api-url` (string): Daiku API URL

## `daiku profile list`

List profiles

Usage: `daiku profile list`

## `daiku profile remove`

Remove a profile and its local credentials

Usage: `daiku profile remove <name>`

Flags:

- `--yes` (bool): skip the interactive confirmation

## `daiku profile use`

Select the active profile

Usage: `daiku profile use <name>`

## `daiku projections`

Manage projection scenarios and rules

Usage: `daiku projections`

## `daiku projections calculate`

Calculate a projection on the server

Usage: `daiku projections calculate`

Flags:

- `--portfolio` (string) (required): portfolio ID
- `--scenario` (string) (required): scenario ID

## `daiku projections retirement`

Calculate retirement readiness on the server

Usage: `daiku projections retirement`

Flags:

- `--portfolio` (string) (required): portfolio ID
- `--scenario` (string) (required): scenario ID

## `daiku projections rule-types get`

Get a projection rule type

Usage: `daiku projections rule-types get <type>`

## `daiku projections rule-types list`

List available projection rule types

Usage: `daiku projections rule-types list`

## `daiku projections rules create`

Create a projection rule

Usage: `daiku projections rules create`

Flags:

- `--category` (string) (required): asset, debt, income or expense
- `--config` (string) (required): JSON object using config_fields from daiku projections rule-types get <type>
- `--display-order` (int): display order
- `--enabled` (bool): enable rule
- `--scenario` (string) (required): scenario ID
- `--type` (string) (required): server rule type; inspect with daiku projections rule-types list

## `daiku projections rules delete`

Delete a projection rule

Usage: `daiku projections rules delete <id>`

Flags:

- `--scenario` (string) (required): scenario ID
- `--yes` (bool): skip the interactive confirmation

## `daiku projections rules list`

List projection rules

Usage: `daiku projections rules list`

Flags:

- `--scenario` (string) (required): scenario ID

## `daiku projections rules update`

Update a projection rule

Usage: `daiku projections rules update <id>`

Flags:

- `--category` (string): asset, debt, income or expense
- `--config` (string): JSON object using config_fields from daiku projections rule-types get <type>
- `--display-order` (int): display order
- `--enabled` (bool): enable rule
- `--scenario` (string) (required): scenario ID
- `--type` (string): server rule type; inspect with daiku projections rule-types list

## `daiku projections scenarios create`

Create a projection scenario

Usage: `daiku projections scenarios create`

Flags:

- `--active` (bool): make scenario active
- `--birth-year` (int): birth year
- `--color` (string): display color
- `--display-order` (int): display order
- `--name` (string) (required): scenario name
- `--portfolio` (string) (required): portfolio ID

## `daiku projections scenarios delete`

Delete a projection scenario

Usage: `daiku projections scenarios delete <id>`

Flags:

- `--portfolio` (string) (required): portfolio ID
- `--yes` (bool): skip the interactive confirmation

## `daiku projections scenarios list`

List projection scenarios

Usage: `daiku projections scenarios list`

Flags:

- `--portfolio` (string) (required): portfolio ID

## `daiku projections scenarios update`

Update a projection scenario

Usage: `daiku projections scenarios update <id>`

Flags:

- `--active` (bool): make scenario active
- `--birth-year` (int): birth year
- `--clear-birth-year` (bool): clear the birth year
- `--color` (string): display color
- `--display-order` (int): display order
- `--name` (string): scenario name
- `--portfolio` (string) (required): portfolio ID

## `daiku recurring`

Manage recurring templates and their occurrences

Usage: `daiku recurring`

## `daiku recurring create`

Create a recurring template

Usage: `daiku recurring create`

Flags:

- `--account` (string): source account ID
- `--active` (bool): whether the template is active
- `--amount` (string) (required): amount
- `--category` (string): category ID
- `--creation-mode` (string) (required): creation mode: auto or confirm
- `--currency` (string) (required): ISO currency published by the API contract
- `--day` (int) (required): day of month (1-31)
- `--description` (string) (required): description
- `--destination-account` (string): destination account ID for transfers
- `--frequency` (string) (required): frequency: monthly or yearly
- `--household` (string) (required): household ID
- `--month` (int): month of year for yearly templates (1-12)
- `--start-date` (string): start date (YYYY-MM-DD)
- `--type` (string) (required): transaction type: expense, income or transfer

## `daiku recurring delete`

Delete a recurring template

Usage: `daiku recurring delete <id>`

Flags:

- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku recurring list`

List recurring templates

Usage: `daiku recurring list`

Flags:

- `--household` (string) (required): household ID

## `daiku recurring occurrences confirm`

Confirm an occurrence as a human entry

Usage: `daiku recurring occurrences confirm <id>`

Flags:

- `--amount` (string) (required): final amount
- `--date` (string) (required): final date (YYYY-MM-DD)
- `--household` (string) (required): household ID
- `--to-amount` (string): final destination amount for transfers
- `--yes` (bool): skip the interactive confirmation

## `daiku recurring occurrences list`

List recurring occurrences

Usage: `daiku recurring occurrences list`

Flags:

- `--household` (string) (required): household ID
- `--status` (string): occurrence status

## `daiku recurring occurrences skip`

Skip a pending occurrence

Usage: `daiku recurring occurrences skip <id>`

Flags:

- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku recurring occurrences snooze`

Snooze a pending occurrence

Usage: `daiku recurring occurrences snooze <id>`

Flags:

- `--household` (string) (required): household ID
- `--until` (string) (required): reminder date (YYYY-MM-DD)

## `daiku recurring update`

Update a recurring template

Usage: `daiku recurring update <id>`

Flags:

- `--account` (string): source account ID
- `--active` (bool): whether the template is active
- `--amount` (string): amount
- `--category` (string): category ID
- `--clear-account` (bool): clear the source account
- `--clear-category` (bool): clear the category
- `--clear-destination-account` (bool): clear the destination account
- `--clear-month` (bool): clear the month of year
- `--creation-mode` (string): creation mode: auto or confirm
- `--currency` (string): ISO currency published by the API contract
- `--day` (int): day of month (1-31)
- `--description` (string): description
- `--destination-account` (string): destination account ID for transfers
- `--frequency` (string): frequency: monthly or yearly
- `--household` (string) (required): household ID
- `--month` (int): month of year for yearly templates (1-12)
- `--start-date` (string): start date (YYYY-MM-DD)
- `--type` (string): transaction type: expense, income or transfer

## `daiku reports`

Inspect server-calculated portfolio reports

Usage: `daiku reports`

## `daiku reports currency-exposure`

Show server-calculated currency exposure

Usage: `daiku reports currency-exposure`

Flags:

- `--currency` (string): display currency (UYU, USD, EUR, BRL, GBP, ARS, UI, CLP, COP, MXN, PEN, PYG, BOB, VES, GTQ, HNL, CRC, NIO, PAB, DOP)
- `--date` (string): snapshot date (YYYY-MM-DD; server default today)
- `--portfolio` (string): portfolio ID (omit for user-wide aggregate)

## `daiku reports net-worth`

Show the server-calculated net worth series

Usage: `daiku reports net-worth`

Flags:

- `--currency` (string): display currency (UYU, USD, EUR, BRL, GBP, ARS, UI, CLP, COP, MXN, PEN, PYG, BOB, VES, GTQ, HNL, CRC, NIO, PAB, DOP)
- `--end` (string): final month (YYYY-MM; server default current month)
- `--months` (int): number of monthly snapshots (1-60; server default 12)
- `--portfolio` (string): portfolio ID (omit for user-wide aggregate)

## `daiku tags create`

Create tag

Usage: `daiku tags create`

Flags:

- `--color` (string): color
- `--household` (string) (required): household ID
- `--name` (string) (required): name

## `daiku tags delete`

Delete tag

Usage: `daiku tags delete <resource>`

Flags:

- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku tags list`

List tags

Usage: `daiku tags list`

Flags:

- `--household` (string) (required): household ID

## `daiku tags update`

Update tag

Usage: `daiku tags update <resource>`

Flags:

- `--color` (string): color
- `--household` (string) (required): household ID
- `--name` (string): name

## `daiku transactions bulk-create`

Create transactions in bulk

Usage: `daiku transactions bulk-create`

Flags:

- `--file` (string) (required): JSON file, or - for stdin
- `--household` (string) (required): household ID

## `daiku transactions bulk-update`

Update matching transactions in bulk

Usage: `daiku transactions bulk-update`

Flags:

- `--file` (string) (required): JSON file, or - for stdin
- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku transactions create`

Create a transaction

Usage: `daiku transactions create`

Flags:

- `--account` (string): account ID
- `--account-amount` (string): amount posted to the selected account
- `--amount` (string) (required): decimal amount
- `--category` (string): category ID
- `--currency` (string): currency code published by the transaction API contract
- `--date` (string): transaction date (YYYY-MM-DD)
- `--description` (string) (required): description
- `--household` (string) (required): household ID
- `--tag` (stringSlice): tag ID (repeatable)
- `--type` (string): expense or income

## `daiku transactions delete`

Delete a transaction

Usage: `daiku transactions delete ID`

Flags:

- `--household` (string) (required): household ID
- `--scope` (string): this, future, or plan for installments
- `--yes` (bool): skip the interactive confirmation

## `daiku transactions delete-all`

Delete all transactions

Usage: `daiku transactions delete-all`

Flags:

- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku transactions get`

Get a transaction

Usage: `daiku transactions get ID`

Flags:

- `--household` (string) (required): household ID

## `daiku transactions list`

List transactions

Usage: `daiku transactions list`

Flags:

- `--account` (string): account ID
- `--all` (bool): fetch every matching transaction without pagination
- `--category` (string): category ID
- `--currency` (string): transaction currency code
- `--from` (string): inclusive start date
- `--household` (string) (required): household ID
- `--kind` (string): recurring or one-time
- `--month` (int): month 1-12
- `--ordering` (string): newest, oldest, amount_high, or amount_low
- `--page` (int): page number (starts at 1)
- `--page-size` (int): results per page (1-200)
- `--query` (string): search query
- `--tag` (stringSlice): tag ID (repeatable)
- `--to` (string): inclusive end date
- `--type` (string): expense, income, or transfer (omit to include adjustments)
- `--year` (int): four-digit year

## `daiku transactions search`

Search transactions

Usage: `daiku transactions search`

Flags:

- `--account` (string): account ID
- `--all` (bool): fetch every matching transaction without pagination
- `--category` (string): category ID
- `--currency` (string): transaction currency code
- `--from` (string): inclusive start date
- `--household` (string) (required): household ID
- `--kind` (string): recurring or one-time
- `--month` (int): month 1-12
- `--ordering` (string): newest, oldest, amount_high, or amount_low
- `--page` (int): page number (starts at 1)
- `--page-size` (int): results per page (1-200)
- `--query` (string): search query
- `--tag` (stringSlice): tag ID (repeatable)
- `--to` (string): inclusive end date
- `--type` (string): expense, income, or transfer (omit to include adjustments)
- `--year` (int): four-digit year

## `daiku transactions update`

Update a transaction

Usage: `daiku transactions update ID`

Flags:

- `--account` (string): account ID
- `--account-amount` (string): amount posted to the selected account
- `--amount` (string): decimal amount
- `--category` (string): category ID
- `--clear-account` (bool): set account to null
- `--clear-account-amount` (bool): set account_amount to null
- `--clear-category` (bool): set category to null
- `--clear-recurring` (bool): set recurring_expense to null
- `--clear-tags` (bool): replace tag_ids with an empty list
- `--currency` (string): currency code published by the transaction API contract
- `--date` (string): transaction date (YYYY-MM-DD)
- `--description` (string): description
- `--household` (string) (required): household ID
- `--recurring` (string): recurring expense ID
- `--tag` (stringSlice): tag ID (repeatable)
- `--type` (string): expense, income, transfer, or adjustment

## `daiku transfers candidates`

List transfer candidates

Usage: `daiku transfers candidates TRANSACTION_ID`

Flags:

- `--household` (string) (required): household ID

## `daiku transfers convert`

Convert a transaction to a transfer

Usage: `daiku transfers convert TRANSACTION_ID`

Flags:

- `--household` (string) (required): household ID
- `--peer` (string): existing peer transaction ID
- `--peer-amount` (string): peer decimal amount
- `--to-account` (string): destination account ID
- `--yes` (bool): skip the interactive confirmation

## `daiku transfers create`

Create a balanced transfer

Usage: `daiku transfers create`

Flags:

- `--amount` (string) (required): source decimal amount
- `--date` (string): transfer date
- `--description` (string): description
- `--from-account` (string) (required): source account ID
- `--household` (string) (required): household ID
- `--to-account` (string) (required): destination account ID
- `--to-amount` (string): destination decimal amount

## `daiku transfers unlink`

Unlink both transfer legs

Usage: `daiku transfers unlink TRANSACTION_ID`

Flags:

- `--household` (string) (required): household ID
- `--yes` (bool): skip the interactive confirmation

## `daiku version`

Print the Daiku CLI version

Usage: `daiku version`
