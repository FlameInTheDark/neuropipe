# Extract UUIDs

## Purpose

Scan free text for substrings that look like UUIDs and return them as a list. Useful for pulling correlation IDs out of log lines, error messages, or chat transcripts.

## Outputs

- `result` — list of UUID strings found in the input (in order of appearance). Empty list when none are found.

## Capabilities

None.