# To Unix Milliseconds

## Purpose

Names an existing Date timestamp explicitly as Unix milliseconds. Neuropipe already represents Date values as milliseconds since the Unix epoch, so this pure node is a readable pass-through for integrations.

## Input and output

Connect a finite Number to **Timestamp (ms)**. **Unix Milliseconds** returns the same numeric value.

## Failure notes

The source must be a finite Number.

## Example

Use this node before an HTTP request when the receiving API labels its parameter `unix_ms`; the wire remains a Number without any loss of precision.

