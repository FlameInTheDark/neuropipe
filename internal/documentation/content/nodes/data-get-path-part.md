# Get Path Part

## Purpose

Split a filesystem path into its constituent parts without touching the filesystem. Useful before a Copy/Move/Rename node when you want to inspect or rebuild a path dynamically.

Backslash separators are folded into forward slashes before splitting, and `volume` recognizes a Windows drive-letter prefix (e.g. `C:`) on every platform, so a Windows-style path splits the same way everywhere.

## Outputs

- `result.dir` — the directory portion (everything up to the last separator).
- `result.base` — the final path element (file or folder name including extension).
- `result.name` — `base` without the extension.
- `result.ext` — the extension including the leading dot (e.g. `.txt`).
- `result.volume` — the Windows drive-letter prefix (e.g. `C:`), empty when the path has none.

## Capabilities

None.