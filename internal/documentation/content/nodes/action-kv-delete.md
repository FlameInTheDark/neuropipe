# KV Delete

## Purpose

Deletes one or more keys with `DEL` and reports how many actually existed.
Use it to clean up session data, invalidate cache entries produced by earlier
nodes, or remove temporary coordination keys at the end of a run.

## Parameters and results

Supply keys either as a wired **Keys** list pin or through the inspector's list
editor; a wired pin always wins. The **Deleted** output counts the
keys that existed, so deleting a missing key is not an error.
