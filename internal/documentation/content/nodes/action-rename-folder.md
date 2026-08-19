# Rename Folder

## Purpose

Rename a folder within the same parent directory. The **New name** input must be a single path segment (no separators, no `..`).

The node errors if a folder with the new name already exists in the same parent directory, or when the new name matches the current name.

## Outputs

- `result.newPath` — the new absolute path of the renamed folder.

## Capabilities

Requires **file-read** and **file-write** capabilities.