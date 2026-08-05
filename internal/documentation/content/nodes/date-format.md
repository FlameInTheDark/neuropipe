# Format Date

## Purpose

Converts a Date timestamp into text for a report, chat reply, notification, path, or HTTP value. It is a pure formatting node.

## Inputs

- **Timestamp (ms)** is the required Number timestamp.
- **Format** optionally overrides the inspector format through a Text wire.

## Configuration

**Format** uses Go’s reference-time layout, not `YYYY` tokens. For example, `2006-01-02` produces `2026-08-05`, `15:04` produces `14:30`, and `Mon, 02 Jan 2006 15:04:05 MST` produces a readable full date. **Timezone** selects `local` or `utc` for the formatted value.

## Failure notes

The timestamp must be a finite Number in milliseconds. An empty format falls back to `2006-01-02 15:04:05`.

## Example

`Now.Timestamp (ms)` → **Timestamp (ms)**, set Format to `2006-01-02`, then connect **Text** to a report title or notification message.

