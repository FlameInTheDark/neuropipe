# Delete Folder

## Purpose

Recursively delete a folder tree. Set **Fail if missing** to `true` to surface a missing folder as an error; otherwise the node succeeds with `deleted=false`.

## Outputs

- `result.deleted` — `true` when the folder was removed, `false` when it was already missing and the node was allowed to skip.

## Capabilities

Requires **file-write** capability.