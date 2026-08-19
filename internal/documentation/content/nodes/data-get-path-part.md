# Get Path Part

## Purpose

Split a filesystem path into its constituent parts without touching the filesystem. Useful before a Copy/Move/Rename node when you want to inspect or rebuild a path dynamically.

## Outputs

- `result.dir` — the directory portion (everything up to the last separator).
- `result.base` — the final path element (file or folder name including extension).
- `result.name` — `base` without the extension.
- `result.ext` — the extension including the leading dot (e.g. `.txt`).
- `result.volume` — the volume prefix on Windows (e.g. `C:`), empty on Unix.

## Capabilities

None.