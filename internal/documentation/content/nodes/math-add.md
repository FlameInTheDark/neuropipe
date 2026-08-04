# Add

## Purpose

Adds two numeric values and returns their sum. This is a pure node: it has no Exec pins, performs no side effects, and is memoized only within the active run frame.

## Inputs

**A** and **B** accept a connected Number wire or a manual number in the inspector. A connected pin takes priority over its manual value. New Math nodes start at `0` for both inputs.

## Failure notes

Both inputs must be finite numbers. A result that overflows to infinity stops the requesting execution path with a node error.

## Example

`Constant 12` → **A**, `Constant 8` → **B**, then `Add.Result` → `For Loop.Last Index`.
