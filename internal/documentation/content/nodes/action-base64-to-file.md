# Base64 To File

## Purpose

Decode a Base64 string and write the resulting bytes to a local file. Accepts standard, URL-safe, raw standard, and raw URL-safe Base64 variants — the node tries each in order.

## Outputs

- `result.path` — the written file path.
- `result.bytesWritten` — number of bytes written.

## Capabilities

Requires **file-write** capability.