# Read Excel Rows

## Purpose
Reads a named Excel table — or, when no table is set, a sheet range or the
sheet's used range — and emits it as a list of row objects whose keys are the
header names. Raw mode keeps numbers and booleans typed (dates surface as
Excel serial numbers); formatted mode returns every cell exactly as the
workbook displays it. Table mode resolves the table's sheet and range
automatically; sheet and range only apply when no table is set.

## Example
`Read Excel Rows (C:\Work\orders.xlsx, Table1)` → `For Each` over the Rows
pin → `LLM Prompt` summarising each order.
