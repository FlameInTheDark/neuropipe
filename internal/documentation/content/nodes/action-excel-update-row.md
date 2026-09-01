# Update Excel Row

## Purpose
Updates every row of an Excel table whose key column equals the configured
key value by writing the Fields object into that row. With the upsert option
enabled, a missing key appends a new row instead of failing.

Configure **Field pins** in the inspector and each column becomes a data
input pin. A wired pin overrides the Fields object for the same column,
while an unwired row literal only fills columns the object leaves open — so
both sources compose in one update instead of fighting.

## Example
`C:\Work\orders.xlsx` tracks shipments in `Table1`, keyed by the `Order`
column. When a webhook reports a shipment, only two cells should move:
`Status` and `ShippedAt`. Add two rows to the **Field pins** panel —
**Name** must match the column header exactly, **Label** optionally renames
the pin, **Value** is the literal used when no wire is connected:

| Name | Label | Value |
| ---- | ----- | ----- |
| `Status` | Status | shipped |
| `ShippedAt` | Shipped at | (empty) |

Set the key column to `Order`, wire the webhook's order id into **Key
value**, and wire its timestamp into the **Shipped at** pin. The matching
row becomes `A-101, ..., shipped, 2026-09-01 14:05`: the wired pin writes
`ShippedAt`, the unwired `Status` pin falls back to its literal, and every
column without a pin keeps its old value. Fields the object already defines
are only overridden by *wired* pins, never by literals.
