# Type Assert

## Purpose

Narrows an `any` value to an explicit Blueprint V3 type contract without
converting the value. The node validates primitives, lists, maps, and record
fields at runtime; a mismatch stops the run safely.

## Example

`Get Field → Type Assert ({"kind":"record","fields":[{"name":"id","type":{"kind":"string"}}]}) → Build Object`.

Use **Cast** when a primitive must be converted. Use **Type Assert** only when
the value must already satisfy the selected contract.
