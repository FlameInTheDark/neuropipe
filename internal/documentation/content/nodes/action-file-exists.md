# File Exists

## Purpose

Check whether a path exists on the filesystem and is a regular file. Returns a boolean result and a summary record.

## Outputs

- `result` — `true` if the path is a regular file (or a symlink to one).
- `summary.exists` — same boolean, in record form.

## Capabilities

Requires **file-read** capability.