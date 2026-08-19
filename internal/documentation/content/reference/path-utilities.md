# Path utility nodes

Three pure nodes that operate on path strings without touching the filesystem.

## Get Path Part

Splits a path into its constituent parts:

- `dir` — the directory portion (everything up to the last separator)
- `base` — the final path element (file or folder name including extension)
- `name` — `base` without the extension
- `ext` — the extension including the leading dot (e.g. `.txt`)
- `volume` — the volume prefix on Windows (e.g. `C:`), empty on Unix

## Build Path

Joins a list of path parts into a single normalized path using the OS's native path separator. The **Parts** input accepts either a list of strings or a newline-separated string.

## Clean Path

Normalizes a path by removing redundant separators and resolving `.` and `..` segments. Equivalent to Go's `filepath.Clean`.
