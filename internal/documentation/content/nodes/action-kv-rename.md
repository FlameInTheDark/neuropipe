# KV Rename

## Purpose

Renames a key with `RENAME`, or with `RENAMENX` when **Fail when the target
exists** is enabled. Rename is the atomic building block for promoting
staging keys — write data under a temporary key, then rename it into its
final place once it is complete.

## Parameters and results

Both the source **Key** and the **New key** can come from wired pins, so a
pipeline can derive the destination name from data. **Renamed** reports
false when the guarded rename was skipped because the target already existed.
