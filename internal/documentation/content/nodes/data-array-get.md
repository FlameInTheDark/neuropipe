# Pick from Array

## Purpose

Reads the element at a zero-based index from a list.

## Example

`HTTP Request json → Pick from Array → Cast`.

## Notes

The Index must be a whole number within the list bounds. A negative, fractional,
or out-of-range index fails the requesting execution path; combine the Length
node with a Branch to guard dynamic indexes.

You can set the Index in the node's inspector when no Index data pin is
connected; connecting a wire always overrides the configured value.
