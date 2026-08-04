# Subtract

## Purpose

Subtracts numeric input **B** from numeric input **A**. It is a pure, side-effect-free value node with no Exec pins.

## Inputs

Set **A** and **B** directly in the inspector when their pins are not wired. A connected Number wire always takes priority. Both manual values default to `0`.

## Failure notes

Both inputs must be finite numbers. A non-finite result stops the requesting execution path.

## Example

`Get Field current` → **A**, `Get Field baseline` → **B**, then `Subtract.Result` → `Greater Than.Left`.
