# UUID nodes

Neuropipe includes four nodes for working with UUIDs. They live under the **Data** category and use the [google/uuid](https://pkg.go.dev/github.com/google/uuid) library.

## Generate UUID

Produce a UUID in a selected version:

- **v4** — random (default).
- **v1** — time + node.
- **v7** — time-ordered random (suitable for sortable keys).
- **v5** — SHA-1 named (deterministic from a namespace UUID + name string).
- **v3** — MD5 named (deterministic; rarely used today).

For v5/v3, supply the **Namespace** UUID (any valid UUID string; the well-known DNS namespace is `6ba7b810-9dad-11d1-80b4-00c04fd430c8`) and the **Name** string.

## Parse UUID

Parses a UUID and returns its version, variant, raw bytes, and URN form. Invalid input does not fail the node — it returns `isValid=false` so pipelines can branch on the outcome.

## Validate UUID

Boolean validator that also forwards the input as a value pin, useful for chaining before a Switch or Branch.

## Extract UUIDs

Scans free text for substrings that look like UUIDs and returns them as a list, in order of appearance.
