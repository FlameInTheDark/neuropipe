# KV Exists

## Purpose

Counts how many of the given keys exist, using `EXISTS`. It is the cheap
guard to place before a heavy branch: skip re-fetching or re-generating data
that is already cached.

## Parameters and results

Keys arrive on the **Keys** pin or through the inspector's list editor. **Existing
keys** reports the count, and **Exists** is a convenience boolean for
single-key checks wired straight into a Branch node.
