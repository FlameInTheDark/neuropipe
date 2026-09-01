# Delete Excel Row

## Purpose
Deletes the first — or, with the delete-all option, every — row of an Excel
table whose key column equals the configured key value, then shrinks the
table range to match.

## Example
`Cron Trigger` nightly → `Read Excel Rows` → `For Each` expired rows →
`Delete Excel Row (Table1, key column "Order")`.
