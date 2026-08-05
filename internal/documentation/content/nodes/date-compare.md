# Compare Dates

## Purpose

Compares two Date timestamps and produces Booleans suitable for Blueprint control flow. It has no Exec pins; connect one Boolean result to `Branch.Condition` to make the execution decision.

## Inputs

Both **Left (ms)** and **Right (ms)** are required finite Number timestamps in milliseconds.

## Outputs

- **Before**, **After**, and **Equal** are mutually exclusive Boolean results.
- **Difference (ms)** is `Left − Right`.
- The seconds, minutes, hours, and days outputs are the same signed difference in the named unit.

## Failure notes

Both inputs must be Numbers. This node compares instants, so two strings should first be converted with `Parse Date`.

## Example

`Now.Timestamp (ms)` → **Left (ms)** and `Create Date.Timestamp (ms)` → **Right (ms)**. Connect **After** to a Branch to run only after a deadline.

