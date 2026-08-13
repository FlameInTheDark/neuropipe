# SQL

## Purpose

Executes one SQL statement against a registered local SQLite database. Choose a
database, enter SQL, and define named parameters. Parameter values arrive on
typed input pins and are bound without string interpolation.

## Parameters and results

Use names such as `:userId` in the statement and configure a parameter named
`userId`. The node exposes `Columns`, `Rows`, `Rows affected`, `Last insert ID`,
and `Truncated` data outputs, followed by the `Then` execution output.

Queries return at most 500 rows from a node execution. Statements must contain
one operation and use named parameters; positional `?` placeholders and
multiple statements are rejected.
