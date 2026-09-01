# Unique Array

## Purpose

Removes duplicate values from a list, keeping each value's first occurrence
and the order of the survivors. Two elements count as duplicates when they
carry equal JSON content, so objects and nested lists deduplicate too, and
numbers compare numerically — `1` and `1.0` collapse into one entry.

## Example

`KV Set Members → Unique Array → Sort Array → For Each` — a clean, sorted
list of distinct member names.

## Notes

The input list is never mutated. Text stays distinct from numbers: `"1"` and
`1` are different values. An element that cannot be serialized to JSON stops
the requesting execution path; values produced by first-party nodes always
serialize.
