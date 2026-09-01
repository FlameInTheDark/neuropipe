# Build Path

## Purpose

Join a list of path parts into a single normalized path. Useful when constructing a destination path from individual pieces before a Copy/Move/Rename node.

Paths use slash-only semantics, so one pipeline produces the same text on every platform: backslash separators in the parts are folded into forward slashes and the joined result always uses forward slashes (which the Windows file APIs accept).

The **Parts** input accepts either a list of strings or a newline-separated string (so the inspector's textarea works without a custom control).

## Outputs

- `result` — the joined and cleaned path.

## Capabilities

None.