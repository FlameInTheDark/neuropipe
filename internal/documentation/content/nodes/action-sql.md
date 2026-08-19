# SQL

## Purpose

Executes one SQL statement against a registered database (SQLite, PostgreSQL, or
MySQL). Choose a database, enter SQL, and define named parameters. Parameter
values arrive on typed input pins and are bound without string interpolation.

## Parameters and results

Use names such as `:userId` in the statement and configure a parameter named
`userId`. The node exposes `Columns`, `Rows`, `Rows affected`, `Last insert ID`,
and `Truncated` data outputs, followed by the `Then` execution output.

Queries return at most 500 rows from a node execution. Statements must contain
one operation and use named parameters; positional `?` placeholders and
multiple statements are rejected.

## Supplying the statement dynamically

The node always exposes an **SQL** input pin. When that pin is connected, the
wired value (a string) replaces the editor-configured statement — letting
another node produce the SQL at run time, for example an LLM Prompt, an HTTP
result, or a file reader. When the pin is not connected, the SQL authored
through **Edit SQL** is used.

The reserved pin ID `sql` cannot be reused for a configured parameter. Choose a
different name for any user-defined parameter so it does not collide with the
node's SQL pin.

