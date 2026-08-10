# Regex Replace

## Purpose

Replaces every Go RE2 match in Text. **Pattern** and **Replacement** can be
configured in the inspector or supplied by connected Text wires; a connected
wire takes precedence.

## Outputs

- **Text** is the replaced text.
- **Replacements** is the exact integer number of matched expressions.
- **Changed** reports whether the output text differs from the input text.

No match is not an error. The original Text is returned with zero Replacements
and Changed set to false.

## Example

With Text `Ada Lovelace`, Pattern `(?P<first>\w+) (?P<last>\w+)`, and
Replacement `${last}, $1`, the output Text is `Lovelace, Ada`.

## RE2 and replacement syntax

Patterns use Go's safe RE2 syntax. Replacement expansion follows Go's
`regexp.ReplaceAllString`: `$1` addresses the first capture and `${name}` a
named capture. Lookaround and pattern backreferences are unsupported. An
invalid pattern fails the node; values are never implicitly converted.
