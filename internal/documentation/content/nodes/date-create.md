# Create Date

## Purpose

Builds one Date timestamp from calendar and clock components. It is pure: a connected input overrides its inspector value and it performs no action by itself.

## Inputs

Set **Year**, **Month**, **Day**, **Hour**, **Minute**, **Second**, and **Millisecond** in the inspector or connect Number values. New nodes start with the current year, January 1, and midnight.

## Outputs

- **Timestamp (ms)** is milliseconds since the Unix epoch.
- **ISO 8601** is the created value as RFC3339 text.

## Configuration

**Timezone** determines how the supplied calendar values are interpreted. Use `utc` for stable, shareable dates and `local` for an event that is intentionally based on this PC’s local time.

## Failure notes

Month must be between 1 and 12. Calendar and clock inputs should be whole values in their normal ranges; invalid calendar combinations are currently normalised by Go’s time library.

## Example

Set Year `2026`, Month `12`, Day `31`, Hour `9`, and Timezone `local`. Connect **Timestamp (ms)** to `Format Date.Timestamp (ms)` to generate a reminder label.

