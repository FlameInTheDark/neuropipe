# Regex Split

## Purpose

Splits Text at every Go RE2 match. Set **Pattern** in the inspector or connect
a Text wire to override the saved value for the current run.

## Outputs

- **Parts** is an exact `list[string]`.
- **Splits** is the exact integer number of delimiter matches.
- **Matched** reports whether the pattern appeared in the input.

Leading and trailing empty parts are retained. When the pattern does not
match, Parts is a one-item list containing the original Text, Splits is zero,
and Matched is false.

## Example

Use Text `one, two; three` with Pattern `[,;]\s*`. Parts is
`["one", "two", "three"]`, which can be passed directly to **For Each**.

## RE2 syntax

Patterns use Go's safe RE2 syntax, not PCRE. Lookahead, lookbehind, and
pattern backreferences are unsupported. Invalid expressions fail explicitly;
the node never coerces non-Text values into text.
