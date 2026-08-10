# Regex Match

## Purpose

Tests Text with a Go RE2 regular expression and returns every match as a typed
structure. Connect **Text** and set **Pattern** in the inspector, or connect a
Text wire to Pattern to override the saved value for that run.

## Outputs

- **Matched** is true when at least one match exists.
- **Count** is the exact integer number of matches.
- **Matches** is `list[RegexMatch]`. Each match contains `text`, `startByte`,
  `endByte`, and `captures`.

Each capture is a `RegexCapture` with a one-based `index`, its `name` (empty
for an unnamed group), `matched`, `text`, `startByte`, and `endByte`. Optional
groups that do not participate are safe values: `matched` is false, text is
empty, and both offsets are `-1`. Offsets are UTF-8 byte positions, the same
unit used by Go's `regexp` package.

No match is not an error: Matched is false, Count is zero, and Matches is an
empty list.

## Example

Use Pattern `(?P<name>\w+)=(?P<value>\d+)` with Text `limit=25 retries=3`.
Matches contains two records. The first has `text` `limit=25`; its captures
are named `name` (`limit`) and `value` (`25`). Connect Matches to **For Each**
or use **Get Field** to expose a selected part of the structure.

## RE2 syntax

Patterns use Go's safe RE2 engine. Named groups use `(?P<name>...)`. Lookahead,
lookbehind, and pattern backreferences are not supported. An invalid pattern
fails the node without converting or guessing the expression.
