# KV Set Add

## Purpose

Adds members to a set with `SADD` and reports how many were new. Sets are the
idempotent collection primitive: adding the same member twice is harmless,
which makes the node ideal for tag lists, seen-sets, and deduplication.

## Parameters and results

Members arrive through the inspector's list editor or a wired **Members** pin.
**New members** counts only the members that were not already present.
