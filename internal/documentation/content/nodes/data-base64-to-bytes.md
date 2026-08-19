# Base64 To Bytes

## Purpose

Decode a Base64 string to raw bytes, auto-detecting standard, URL-safe, raw standard, and raw URL-safe variants. The output is bytes (no wire-representation picker), so it can be wired directly into bytes-only pins (e.g. Write File with `bytes` content).

## Outputs

- `result` — the decoded bytes.

## Capabilities

None.