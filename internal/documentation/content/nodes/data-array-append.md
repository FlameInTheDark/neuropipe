# Append to Array

## Purpose

Appends onto a list and produces the new list. The node is pure: it never
modifies the connected array, so the same array pin remains reusable
elsewhere.

The **Append** mode decides what the Value input means. **Single item**
appends the value as one element — wiring a list here nests it as a single
list element. **Array elements** concatenates: the wired list's elements are
appended one after another, so appending `[3, 4]` onto `[1, 2]` yields
`[1, 2, 3, 4]`, the way one array is appended to another in a typed language.

## Example

`HTTP Request json → Append to Array (Array elements) → For Each Loop` —
merge a freshly fetched page of results onto an accumulated list.

## Notes

The input Array must be a list; anything else fails the requesting execution
path. In **Array elements** mode the Value input must also be a list, and a
non-list value fails with that named. Chain multiple Append to Array nodes
to build or merge lists step by step; every append produces a fresh list, so
earlier nodes in the chain keep their own output intact.
