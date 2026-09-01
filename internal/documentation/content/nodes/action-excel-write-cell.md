# Write Excel Cell

## Purpose
Writes one value into a worksheet cell by its A1 reference. Text starting
with `=` is stored as a formula, so `=SUM(A1:A9)` writes a live formula.
Numeric text is converted to a number cell unless the conversion option is
disabled.

Configure **Cell pins** in the inspector and several cells become data input
pins at once: a wired value lands in its cell, an unwired row literal is the
fallback, and the single Cell/Value fields become optional. This is the
natural way to stamp a handful of computed values into a report sheet in
one node.

## Example
At the end of a run, `C:\Work\report.xlsx` (sheet `Dashboard`) should receive
the summary text, the order total, and a timestamp. Add three rows to the
**Cell pins** panel — **Name** is the A1 cell reference, **Label** optionally
renames the pin on the canvas, **Value** is the literal used when no wire is
connected:

| Name | Label | Value |
| ---- | ----- | ----- |
| `B2` | Summary | Weekly digest |
| `B3` | Total | 0 |
| `B4` | Stamp | (empty) |

Wire the `LLM Prompt` answer into **Summary**, a math result into **Total**,
and a date node into **Stamp**. One execution writes all three cells — B2
the wired text, B3 the wired number, B4 the wired timestamp — and an unwired
pin would simply fall back to its literal. The classic single Cell and Value
fields stay empty while pins do the work; fill them only when the same run
should also write one extra fixed cell.
