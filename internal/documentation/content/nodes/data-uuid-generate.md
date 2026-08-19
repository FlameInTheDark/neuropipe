# Generate UUID

## Purpose

Generate a UUID in a selected version: **v4** (random, default), **v1** (time + node), **v7** (time-ordered random), **v5** (SHA-1 named), or **v3** (MD5 named).

v3 and v5 require both a **Namespace** UUID (any valid UUID string, typically the well-known DNS namespace `6ba7b810-9dad-11d1-80b4-00c04fd430c8`) and a **Name** string. The same `(namespace, name)` pair always produces the same UUID for v5/v3.

## Outputs

- `value` — the generated UUID in lowercase canonical form.

## Capabilities

None.