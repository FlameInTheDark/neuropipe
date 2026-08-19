# Move File

## Purpose

Move a file to a new path. Tries an atomic `rename` first; if the source and target live on different filesystems, silently falls back to copy-then-delete.

## Outputs

- `result.targetPath` — the cleaned target path.

## Capabilities

Requires **file-read** and **file-write** capabilities.