# Add Duration

## Purpose

Adds calendar and clock amounts to a Date timestamp. Years, months, and days use calendar arithmetic; hours through milliseconds add elapsed time. The node is pure.

## Inputs

Connect **Timestamp (ms)** and optionally provide **Years**, **Months**, **Days**, **Hours**, **Minutes**, **Seconds**, and **Milliseconds**. Duration fields can also be set manually in the inspector; connected values take priority.

## Outputs

- **Timestamp (ms)** is the resulting instant.
- **ISO 8601** is the resulting RFC3339 text.

## Configuration

**Timezone** chooses the timezone used while applying calendar units. Use UTC when date arithmetic must be identical on every computer.

## Failure notes

The timestamp must be a finite Number. Calendar values are whole-number components; adding months around the end of a month follows the runtime’s calendar-normalisation behavior.

## Example

Connect `Now.Timestamp (ms)` to **Timestamp (ms)**, set Days to `7`, then format the output as a one-week reminder date.

