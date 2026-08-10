# Get Type

## Purpose

Reports the JSON type of any value: `text`, `number`, `boolean`, `object`,
`list`, or `null`. When the value is a list, the Element Type output reports the
common element type.

## Example

`HTTP Request json → Get Type → Branch`.

## Notes

For a list, Element Type is `any` when the list is empty, `mixed` when its
elements are not all the same kind, and otherwise one of the JSON types above.
For non-list values Element Type is empty.
