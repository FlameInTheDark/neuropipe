# Text nodes

Text nodes operate on exact Text values. They never parse numbers or coerce
other data to text. Connect a Text wire to override a saved inspector value.

## Split and Join

**Split** returns `list[string]` and Count. Empty segments are preserved, so
splitting `a,,b` on `,` produces `a`, an empty string, and `b`. **Join** accepts
only `list[string]` and joins entries with Separator.

## Comparison and replacement

**Contains**, **Starts With**, and **Ends With** compare exact Text values and
return Bool. They are case-sensitive; use **Change Case** first when needed.
**Replace** changes the first match, an exact positive Count, or all matches.
It returns the result, the number of replacements, and Changed. An empty Find
value fails explicitly.

## Trim, case, and Unicode ranges

**Trim** removes Unicode whitespace. **Change Case** returns lower, upper, or
Unicode title case. **Index Of** returns a Unicode code-point offset and Found;
an absent value has Index `-1`. **Substring** takes code-point Start and Length.
Negative or out-of-range values fail explicitly. For example,
`Substring("héllo", 1, 2)` returns `él`, not a UTF-8 byte fragment.
