# Document nodes

Neuropipe provides Word and Excel Blueprint nodes that operate on local
.docx and .xlsx files — no Office installation and no cloud account
required. All nodes live under the **Documents** category in the library
panel.

## Dynamic value pins

The Documents nodes expose **configurable value pins**: open the node in the
inspector and add rows to its *Value pins* / *Cell pins* / *Column pins* /
*Field pins* panel. Every row becomes an ordinary data pin on the node, so
upstream outputs — an LLM answer, a Constant, a math result — wire straight
into a placeholder, cell, or column without assembling a JSON object first.

- **Populate Word Template**: one input pin per `{{placeholder}}`.
- **Write Excel Cell**: one input pin per worksheet cell.
- **Read Excel Cell**: one *output* pin per worksheet cell.
- **Append Excel Rows**: one input pin per table column; each run appends
  one row assembled from the pins.
- **Update Excel Row**: one input pin per updated column.

Each row carries an optional label (the pin name on the canvas) and an
optional literal used when no wire feeds the pin. When both a pin and the
classic object input (Values, Fields, Rows JSON) are used, a *wired* pin
overrides the object entry for the same key, while an unwired literal only
fills keys the object leaves open — so the two sources compose instead of
fighting. Pins accept any wire type: placeholders are stringified, cells
and columns coerce numeric text exactly like the single-value fields. Cell
references are validated when the editor commits them, so typos surface
before a run. The static single-value pins (Cell, Value, Rows, Fields)
become optional while pins are configured.

## Excel nodes

Excel workbooks are addressed by file path; tables are addressed by their
workbook-unique table name.

- **Read Excel Rows** — emit a table, sheet range, or used range as a list
  of row objects. Raw mode keeps numbers and booleans typed; formatted mode
  returns displayed text.
- **Append Excel Rows** — append one or many row objects, extending the
  table range, with optional table and workbook auto-creation. With no
  table configured, rows are appended to the sheet using row 1 as the
  header row. Column pins append one additional row assembled directly from
  wired values.
- **Update Excel Row** — rewrite every row whose key column equals a value,
  with an optional upsert when nothing matches.
- **Delete Excel Row** — remove the first or every matching row and shrink
  the table range.
- **Read Excel Cell** / **Write Excel Cell** — read or write one A1 cell;
  values starting with `=` become live formulas. Both also expose
  configured cells as pins: writes take one input pin per cell, reads emit
  one output pin per cell.
- **List Excel Worksheets** / **Add Excel Worksheet** — enumerate a
  workbook's sheets in order or create a new one.

## Word nodes

Word documents are read and written through a built-in WordprocessingML
engine, so no Office installation is required. Mutating nodes rewrite only
the paragraphs that actually change and keep the rest of the document
byte-identical.

- **Read Word Text** — extract paragraphs, line breaks, and tables as plain
  text.
- **Create Word Document** — a bold title plus one paragraph per line.
- **Populate Word Template** — replace `{{placeholder}}` fields from a
  values object and save the result as a new document. Unknown placeholders
  are preserved so missing data stays visible. Value pins wire one
  placeholder each without a Build Object detour.
- **Append to Word Document** — append one paragraph per line.
- **Replace Word Text** — find and replace across body, headers, and footers.

## Scope notes

A few document features are deliberately absent: converting Word documents
to PDF, embedded macros and scripts, and workbook comments. Word template
fill uses plain-text placeholders instead of Word content controls, and
repeating sections are not supported — generate one document per row with a
loop instead. Reading Excel dates in raw mode yields Excel serial numbers;
use formatted mode or the Date nodes to convert them.
