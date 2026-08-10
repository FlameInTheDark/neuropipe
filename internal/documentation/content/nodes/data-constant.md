# Constant

## Purpose
Provides a literal value without exec flow.

## Configuration
- **Value**: the literal value as text.
- **Type**: how the Value is interpreted — `text`, `number`, or `boolean`.
  Number and Boolean values are parsed from the text Value; an unparseable
  Value fails the run.

## Example
Set Value to `5` with Type `number` and connect Constant to For Loop's Last
Index. The output pin type follows the selected Type (`text`, `number`, or
`boolean`), so the editor highlights incompatible connections accordingly.
