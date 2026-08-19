# Wait For Folder

## Purpose

Poll a path until a directory exists at it, or until the timeout elapses. Mirrors Wait For File but checks for directories.

## Outputs

- `found` — execution output fired when the folder appears before the deadline.
- `timeout` — execution output fired when the deadline is reached.
- `result.found` / `result.waitedSeconds`.

## Capabilities

Requires **file-read** capability.