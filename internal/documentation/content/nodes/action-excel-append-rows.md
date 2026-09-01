# Append Excel Rows

## Purpose
Appends one row object or a list of row objects to an Excel table, extending
the table range to cover them. When the table does not exist yet it can be
created from the row keys, and when the workbook does not exist yet a new
one can be written. With no table configured, rows are appended below the
sheet's existing content using row 1 as the header row.

Configure **Column pins** in the inspector — one input pin per table column.
Each run appends one additional row assembled from the pins: wired values
first, per-row literals as fallback, absent columns skipped. Rows supplied
through the Rows input or JSON land in the same run, so a full list and one
computed row can be appended together.

## Example
`C:\Work\orders.xlsx` contains the table `Table1` with the columns `Order`,
`Item`, and `Amount`. A webhook delivers one order per run and every order
must be logged. Add three rows to the **Column pins** panel — **Name** must
match the column header exactly, **Label** optionally renames the pin,
**Value** is the literal used when no wire is connected:

| Name | Label | Value |
| ---- | ----- | ----- |
| `Order` | Order | A-101 |
| `Item` | Item | (empty) |
| `Amount` | Amount | 0 |

Wire the webhook's parsed fields (or `Get Field` outputs) into **Order**,
**Item**, and **Amount**. Each execution appends one row such as
`A-101, USB-C cable, 19.9`: wired values win, unwired pins fall back to
their literals. A nightly batch flow can instead wire a list into the Rows
input and append hundreds of rows in one execution, with the pin row added
on top.
