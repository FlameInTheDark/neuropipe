# Delete File

## Purpose

Delete a regular file. Set **Fail if missing** to `true` to surface a missing file as an error; otherwise the node succeeds with `deleted=false`.

## Outputs

- `result.deleted` — `true` when the file was removed, `false` when it was already missing and the node was allowed to skip.

## Capabilities

Requires **file-write** capability.