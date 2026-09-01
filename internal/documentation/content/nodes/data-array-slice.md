# Slice Array

## Purpose

Cuts a section out of a list: skip **Start** elements, then take **Count**
elements. Without a Count the slice runs to the end of the list, so a Start
alone is a skip operation.

## Example

`Read Excel Rows rows → Slice Array (Start 0, Count 20) → For Each` —
paginate a result set twenty elements at a time; raise Start by twenty for
each page.

## Notes

Start defaults to zero. Both settings accept a wire or the inspector value,
and a wire always wins. Start and Count must be whole numbers; a Start past
the end yields an empty list and a Count past the end is clamped, so slicing
never fails on bounds. Use Pick from Array for a single element instead of a
section.
