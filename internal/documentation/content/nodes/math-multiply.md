# Multiply

## Purpose

Multiplies two numeric values. It is evaluated only when another node requests its Result data pin and has no Exec pins or capabilities.

## Inputs

**A** and **B** can be supplied by Number wires or edited directly in the inspector. Wires override the corresponding manual value; both manual values start at `0`.

## Failure notes

Both inputs must be finite numbers. Overflow is rejected instead of returning an infinite value.

## Example

`Get Field price` → **A**, `Constant 1.2` → **B**, then `Multiply.Result` → `Create Report`.
