# Move Folder

## Purpose

Move a folder tree to a new path. Tries an atomic `rename` first; if the source and target live on different filesystems, recursively copies then deletes the source. Symlinks are skipped during the cross-device copy.

## Outputs

- `result.targetPath` — the cleaned target path.
- `result.filesMoved` — number of files moved during the cross-device fallback (zero for an atomic rename).

## Capabilities

Requires **file-read** and **file-write** capabilities.