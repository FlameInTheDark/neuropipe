# Clean Path

## Purpose

Normalize a filesystem path by removing redundant separators and resolving `.` and `..` segments. Equivalent to Go's `path.Clean` on the slash-separated form: backslashes are folded into forward slashes first, so `C:\Work\..\Work\file.txt` normalizes to `C:/Work/file.txt` identically on every platform.

## Outputs

- `result` — the cleaned path.

## Capabilities

None.