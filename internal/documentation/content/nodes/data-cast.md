# Cast

## Purpose

Explicitly converts a value to text, number, Boolean, object, list, or bytes.

## Example

`Pick from Array → Cast (object) → KV Hash Set.Fields`.

## Notes

Objects and lists pass through unchanged when the value already has the target
shape; JSON text parses into that shape. Casting to text serializes objects and
lists as compact JSON and decodes bytes as raw text; casting to bytes encodes
text as raw bytes. Invalid conversions — a number to object, an object to list —
fail the requesting execution path instead of guessing.
