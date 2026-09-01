# Parse JSON

## Purpose

Parses JSON text into a typed value. Set **Root type** to **object** (the
default), **list**, **text**, **number**, or **boolean** and the Value pin
carries that wire contract, so the parsed value connects to typed inputs
without a Cast. **any** keeps the historical untyped contract for mixed roots.

## Example

`Read File content → Parse JSON (Root type object) → Word Template Fill
Values` — no Cast node in between.

## Notes

Objects use the graph-wide `map<string, any>` shape, the same contract as
Cast's object target, Build Map, and Word/Excel value pins. A parsed root
that does not match the declared type fails loudly: parsing `[1, 2]` with
Root type object stops the requesting execution path and names both kinds,
with the fix in the message. Graphs saved before Root type existed keep the
untyped any contract until you pick a type, so their wires stay valid.
Malformed JSON stops the requesting path.
