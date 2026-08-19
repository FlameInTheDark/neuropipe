# Rename File

## Purpose

Rename a file within the same directory. The **New name** input must be a single path segment (no separators, no `..`) — the node rejects attempts to escape the file's directory.

The node errors if a file with the new name already exists in the same directory.

## Outputs

- `result.newPath` — the new absolute path of the renamed file.

## Capabilities

Requires **file-read** and **file-write** capabilities.