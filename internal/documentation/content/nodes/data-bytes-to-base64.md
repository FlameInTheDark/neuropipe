# Bytes To Base64

## Purpose

Encode connected raw bytes as a Base64 string. This is the type-strict variant of Base64 Encode — its input is always bytes (no wire-representation picker), which makes its pin contract unambiguous when wiring it against bytes-only sources.

## Outputs

- `result` — the Base64-encoded string.

## Capabilities

None.