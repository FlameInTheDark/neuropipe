# Now

## Purpose

Produces the current instant as a Date timestamp. It is a pure node, so it has no Exec pins and is evaluated once only when a connected output is requested during a run.

## Outputs

- **Timestamp (ms)** is the canonical Date value: milliseconds since the Unix epoch.
- **ISO 8601** is an RFC3339 text representation of that instant.
- **Local String** is a human-readable `YYYY-MM-DD HH:MM:SS` value in the selected timezone.

## Configuration

Choose **Timezone**: `local` uses the computer timezone and `utc` uses UTC. The choice changes only the displayed text fields; the timestamp still identifies the same instant.

## Example

Connect **Timestamp (ms)** to `Compare Dates.Left (ms)` and a fixed `Create Date.Timestamp (ms)` to the right input. Connect **After** to a Branch condition to run a reminder only after the chosen date.

