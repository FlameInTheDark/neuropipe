# Build Path

## Purpose

Join a list of path parts into a single normalized path using the OS's native path separator. Useful when constructing a destination path from individual pieces before a Copy/Move/Rename node.

The **Parts** input accepts either a list of strings or a newline-separated string (so the inspector's textarea works without a custom control).

## Outputs

- `result` — the joined and cleaned path.

## Capabilities

None.