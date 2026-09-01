# Build Array

## Purpose
Assembles an array from configurable input pins. Like an array in a typed
language, every element shares one **Element type**: choose it once on the
node and every pin, every constant, and the **Array** output carry it, so a
number array cannot quietly pick up a text value. Choose **any** when the
elements are genuinely mixed — that is the only mode where different types
may flow side by side.

Each row in **Items** is one input pin under a stable ID: renaming a pin or
moving it up and down keeps its wires attached, and reordering rows
reorders the elements. A row's optional **constant** fills the element when
no wire lands and is validated against the element type; a row with neither
a wire nor a constant fails the run naming the item instead of silently
inserting null. The output contract follows the element type — `list<T>`
when concrete, `list<any>` for any — so a typed wire into a mismatched
consumer is rejected during validation, while any-typed inputs accept every
array.

## Example
A nightly report needs three text slots: a fixed title plus two wired
values. Set **Element type** to **text**, open the item editor, and
configure the rows — **Pin name** labels the pin on the canvas, and
**Constant** fills the slot when no wire lands:

| Pin name | Constant |
| -------- | -------- |
| Title | Weekly digest |
| Total | (empty) |
| Date | (empty) |

Wire a formatted total into **Total** and a date node into **Date**. The
output is `["Weekly digest", "42", "2026-09-01"]` — the constant fills the
first slot, wires fill the rest, and every element is text. Feed the
**Array** output into For Each, KV List Push, or an HTTP JSON body. When
the elements must be mixed (a number next to texts), set **Element type**
to **any** instead.
