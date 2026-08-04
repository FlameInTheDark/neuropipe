# Divide

## Purpose

Divides numeric input **A** by numeric input **B**. It is a pure data node with no Exec pins and no capabilities.

## Inputs

Enter manual values for **A** and **B** in the inspector when those pins are not connected. A Number wire takes precedence over a manual value. Both inputs default to `0`.

## Failure notes

Both inputs must be finite numbers and **B** must not be zero. Invalid input, division by zero, and a non-finite result stop the requesting execution path.

## Example

`Get Field completed` → **A**, `Get Field total` → **B**, then `Divide.Result` → `Format Text`.
