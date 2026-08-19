# Parse UUID

## Purpose

Parse a UUID string and surface its version, variant, raw bytes, and URN form. Invalid input does not fail the node — it returns `isValid=false` so pipelines can branch on the outcome.

## Outputs

- `result.isValid` — boolean.
- `result.version` — integer (1, 2, 3, 4, 5, 6, 7, or 0 for invalid).
- `result.variant` — `rfc4122`, `microsoft`, `future`, `reserved`, or `unknown`.
- `result.bytes` — the 16 raw bytes of the UUID.
- `result.urn` — the URN form (`urn:uuid:...`).

## Capabilities

None.