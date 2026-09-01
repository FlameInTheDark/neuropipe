# Build Map

## Purpose
Assembles a flat map object — string keys to values of one shared **Value
type**, the dictionary counterpart of Build Object (whose dotted keys
construct nested objects). Keys are used exactly as written: dots, spaces,
and capitalization stay literal and nothing nests. Duplicate keys are
rejected when the editor commits them. Choose **any** as the value type
only when the entries must hold mixed types — like a `map[string]any`.

Each row in **Entries** is one typed input pin with an optional constant
for when no wire is connected; a row with neither fails the run naming the
entry. Row identity is stable: renaming a pin or its key keeps wires
attached. The output contract is `map<string, T>` when the value type is
concrete, so typed consumers validate their wiring instead of discovering
mismatches at runtime.

## Example
An HTTP POST body needs `{"id": "A-101", "currency": "EUR"}` — both text.
Set **Value type** to **text** and configure two rows — **Key** becomes the
object key verbatim, **Pin name** labels the pin on the canvas, and
**Constant** fills the entry when no wire lands:

| Key | Pin name | Constant |
| --- | -------- | -------- |
| id | Id | (empty) |
| currency | Currency | EUR |

Wire the order id into **Id**. The **Map** output is `{"id": "A-101",
"currency": "EUR"}` — the wired pin fills its entry, the unwired currency
falls back to its constant, and every value is text. Use the output as a
JSON body, a KV value, or a Fields payload. Mixed values (a number total
next to texts) require **any**.
