# Sort Array

## Purpose

Returns a sorted copy of a list. Numbers sort numerically, text sorts
lexicographically, and Booleans sort false before true. Equal elements keep
their original order, so the result is stable and predictable.

Set **Order** to **ascending** (the default) or **descending**.

## Example

`Read Excel Rows totals → Sort Array (descending) → Pick from Array (Index 0)`
— surface the largest value first.

## Notes

Mixed scalar lists sort by type: all numbers first, then all text, then all
Booleans. Objects, lists, bytes, and null cannot be ordered — they stop the
requesting execution path with the kind named. To sort objects, extract a
scalar key first (Get Field or Break Object), sort it, and recombine. The
input list is never mutated; sorting produces a fresh array.
