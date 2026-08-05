# To Unix Seconds

## Purpose

Converts Neuropipe’s canonical millisecond timestamp into Unix seconds. This pure node is primarily useful for APIs that expect seconds rather than milliseconds.

## Input and output

Connect a finite Number to **Timestamp (ms)**. **Unix Seconds** returns the timestamp divided by `1000`; milliseconds are retained as a fractional second when present.

## Failure notes

The source must be a finite Number timestamp. Use `Extract Components.Unix Seconds` when you specifically need the integer Unix-second component.

## Example

Connect `Now.Timestamp (ms)` to this node, then pass **Unix Seconds** to an HTTP request query field that accepts a Unix epoch value.

