# Random Number

## Purpose
Generate a random number on each Blueprint execution, with optional range bounds and a choice of float or integer output.

## Configuration
- **Type**: `float` for a fractional value in `[0, 1)` (or your range), `integer` for a whole number.
- **Use range**: when enabled, the node samples inside `[From, To]` instead of `[0, 1)`.
- **From**: the inclusive lower bound of the range.
- **To**: the inclusive upper bound of the range.

## Inputs
The **From** and **To** data pins are optional. When connected, their values
take priority over the inspector fields, so upstream nodes can dynamically
control the range. When the **Use range** toggle is off, the bounds are
ignored.

## Example
`Button Trigger → Random Number (integer, range 1–6) → Display Message` shows
a dice roll result. Connect `Get Variable` to **From** to drive the lower
bound at runtime while keeping the upper bound in the inspector.
