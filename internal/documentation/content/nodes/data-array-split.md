# Split Array

## Purpose

Cuts a list into consecutive batches of a fixed **Size**. The output is a
list of arrays: feed it into For Each and every iteration receives one whole
batch. The final batch may be shorter; an empty list produces no batches.

## Example

`Query JSON $.items → Split Array (Size 25) → For Each → Append Excel Rows`
— write a large result set to a workbook 25 rows at a time.

## Notes

Size defaults to ten when neither the **Size** pin nor the inspector value is
set, and a wire always wins. Size must be a whole number of at least one.
Each batch is itself an array, so a consumer that expects a single element
needs Pick from Array — the usual pattern inside the batch loop.
