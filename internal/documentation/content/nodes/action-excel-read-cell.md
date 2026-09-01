# Read Excel Cell

## Purpose
Reads a single worksheet cell by its A1 reference. Raw mode returns a typed
value (number, boolean, or text); formatted mode returns the displayed text
including number formats. Use it for dashboard totals, status cells, or
single lookup values.

Configure **Cell pins** in the inspector and each referenced cell becomes a
data output pin, so one node can expose a whole row of settings or KPIs to
downstream nodes without one Read Cell node per value. The value mode
applies to every pin.

## Example
`C:\Work\settings.xlsx` (sheet `Config`) stores the run configuration in one
row: B1 holds the alert threshold, B2 the recipient address, B3 the enabled
flag. Add three rows to the **Cell pins** panel — **Name** is the A1 cell
reference, **Label** names the output pin on the canvas:

| Name | Label |
| ---- | ----- |
| `B1` | Threshold |
| `B2` | Recipient |
| `B3` | Enabled |

The node grows three output pins. Wire **Threshold** into a compare
condition, **Recipient** into the mail node's address input, and **Enabled**
into a condition that gates the whole alert branch — one node fans the
entire configuration row out to the pipeline. The classic **Value** output
remains available when a single Cell is set, so a fixed cell and configured
pins can be mixed in the same node.
