# Populate Word Template

## Purpose
Fills `{{placeholder}}` fields of a .docx template and saves the result as a
new document, leaving the template untouched. Placeholders split across Word
runs are matched anyway because each paragraph is merged before
substitution, and unknown placeholders are preserved so missing data stays
visible in the output.

Values arrive from three sources that compose instead of competing: the
**Values** object, the **Value pins** panel, and per-row literals. A wired
pin overrides the Values object entry for the same placeholder, while an
unwired row literal only fills placeholders the object leaves open.

## Example
A template `C:\Work\invoice.docx` contains the line
`Invoice for {{customer}} — total {{amount}} USD, due {{due_date}}.`

Open the node's **Value pins** panel and add one row per placeholder.
**Name** must match the placeholder exactly (without the braces); **Label**
optionally renames the pin on the canvas; **Value** is the literal used when
no wire feeds the pin.

| Name | Label | Value |
| ---- | ----- | ----- |
| `customer` | Customer | Contoso |
| `amount` | Total | 0 |
| `due_date` | Due date | 2026-09-15 |

Wire an `LLM Extract` answer into the **Customer** pin and a math result
into the **Total** pin. The run writes `Invoice for Northwind Traders —
total 1250 USD, due 2026-09-15.` into `invoice-filled.docx` beside the
template: wired pins win over everything else, the unwired `due_date` falls
back to its literal, and a placeholder with neither source keeps its
`{{braces}}` so the gap stays visible.
