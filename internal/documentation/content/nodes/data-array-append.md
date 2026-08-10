# Append to Array

## Purpose

Appends a value to a list and produces the new list. The node is pure: it never
modifies the connected array, so the same array pin remains reusable elsewhere.

## Example

`HTTP Request json → Append to Array → For Each Loop`.

## Notes

Chain multiple Append to Array nodes to build a list element by element. The
input Array must be a list; anything else fails the requesting execution path.
