# Parse Date

## Purpose

Converts Date text into Neuropipe’s millisecond timestamp. It is pure and can be used before date comparisons, scheduling decisions, or formatting in another timezone.

## Inputs

- **Text** is the required non-empty date text.
- **Format** optionally supplies a Go reference-time layout from a connected Text wire.

## Configuration

Set **Format** in the inspector when the source has a known shape, for example `02/01/2006 15:04`. When it is empty, Neuropipe tries common RFC3339, ISO, numeric, and English month-name layouts. **Timezone** is applied to a date text that has no timezone of its own.

## Failure notes

Use an explicit format for ambiguous numeric dates such as `06/07/2026`; automatic parsing must choose one accepted layout. Invalid or empty text stops the requesting execution path.

## Example

Connect a webhook or file Text field to **Text**, set Format to `2006-01-02`, and connect **Timestamp (ms)** to `Compare Dates.Left (ms)`.

