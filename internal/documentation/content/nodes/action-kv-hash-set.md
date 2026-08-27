# KV Hash Set

## Purpose

Writes or removes hash fields. Set mode runs `HSET` with the fields from the
**Fields** object input; remove mode runs `HDEL` with the same field names,
which lets one node shape both directions of a hash schema.

## Parameters and results

Configure fields in the inspector's field/value editor, or wire an object from a
Build Object or JSON Parse node. Field names are applied in sorted order for
deterministic execution records, and **New fields** counts the fields that
were actually created (or removed in remove mode).
