# Query JSON

Reads one value out of parsed JSON data with a JSONPath expression. Connect
any object or list to **Source** — the HTTP Request result, a Parse JSON
output, a webhook payload — type a path into the **JSON path** field in the
inspector, and the node emits the addressed value on **Value**.

Paths use standard JSONPath syntax: `$` stands for the connected source,
`.name` descends into object keys, and `[n]` picks list elements. The
leading `$` is what the classic notation looks like; the node also accepts
the same selectors without it (`geonames[0].lng`), and for simple lookups a
plain dotted path keeps working too.

The fastest way to write a path: open an execution-log entry in the data
inspector, hover the element you want, and click the copy-path button
(the route icon). It copies that element's JSONPath expression —
`$.items[1].customer.name`, `$["x-request-id"]`, or `$` for the whole
payload — ready to paste straight into the **JSON path** field.

## Example

The geonames.org search API answers with a payload like this:

```json
{
  "geonames": [
    { "name": "Saint Petersburg", "lat": 59.9, "lng": 30.25 },
    { "name": "Moscow", "lat": 55.75, "lng": 37.62 }
  ],
  "totalResultsCount": 2
}
```

Pointing the node at that source:

`HTTP Request.Result → Query JSON ($.geonames[0].lng) → Terminal`
picks the longitude of the first result — `30.25`.

`HTTP Request.Result → Query JSON ($.geonames[*].name) → For Each Loop`
collects every result name into a list — `["Saint Petersburg", "Moscow"]` —
ready for the loop to iterate.

## Syntax

| Selector | Reads | Example |
| --- | --- | --- |
| `$` | the connected source itself | `$` |
| `.name` | one object key | `$.totalResultsCount` |
| `['name']` or `["name"]` | one object key, quoted — use it for keys with dots or spaces | `$['www.geonames.org']` |
| `[n]` | one list element, counting from zero | `$.geonames[0]` |
| `[-n]` | one list element counting from the end | `$.geonames[-1]` is the last item |
| `[*]` or `.*` | every element of a list, or every value of an object | `$.geonames[*].name` |
| `[start:end]` | a slice of a list; both ends optional; negatives count from the end | `$.geonames[1:]` skips the first item |
| `[start:end:step]` | a slice with a stride; a negative step walks backwards | `$.geonames[::-1]` reverses the list |
| `[a,b]` | a union of indexes or names, in the given order | `$.geonames[0,2].name` |
| `$..name` | every `name` key at any depth | `$..lng` |
| `[?(condition)]` | the elements that satisfy a condition | `$.geonames[?(@.lng > 35)]` |

Selectors chain freely, so `$.geonames[0]['lng']` and
`$['geonames'][0].lng` mean the same thing as `$.geonames[0].lng`. A key
that contains brackets or dots always goes in quotes — `$['items[0]']`
reads the literal key `items[0]`.

## Filters

A filter keeps the list elements (or object values) for which the condition
holds. Inside the condition, `@` is the element being tested and `$.`
reaches back into the whole source:

- `$.geonames[?(@.lng > 35)]` — keep elements whose `lng` is greater than 35.
- `$.geonames[?(@.name == 'Moscow')]` — text equality; either quote style works.
- `$.geonames[?(@.name != 'Moscow')]` — inequality; a missing key also counts as unequal.
- `$.geonames[?(@.lat < 60 && @.lng > 35)]` — both conditions; combine with `||` for either, `!` for negation, and parentheses for grouping.
- `$.scores[?(@ > 2)]` — a bare `@` tests the element itself, for lists of scalars.
- `$.geonames[?(@.name == $.wanted)]` — compare against a value from the top of the source.
- `$.geonames[?(@.meta.active == true)]` — paths on both sides of the operator; `true`, `false`, and `null` are literal values.

Numbers compare numerically across representations (integers, decimals,
JSON numbers); text compares alphabetically for `<` and `>`. A condition
with no operator — `$.geonames[?(@.lat)]` — keeps elements where the value
exists and is not null, false, zero, or the empty string.

## Result shape

The **Value** output follows the match count: a path that addresses exactly
one value emits that value itself — `$.geonames[0].lng` emits a number,
ready for a math node. A path that matches several values — wildcards,
slices, unions, recursive descent, filters with several hits — emits the
matches as a list. A path that matches nothing emits null, which downstream
nodes treat as an empty value; check the run log if a pick comes back null
and inspect the source's exact key names.

## Notes

Plain dotted paths from older graphs keep their original meaning:
`geonames.0.lng` still picks the first element's `lng`, with each
dot-separated part acting as a key or, failing that, a list index. New
graphs should prefer JSONPath — it handles nested lists, quoted keys, and
multi-value selectors that the dotted form cannot express.

The evaluator walks structured values directly, so it also reads first-party
results like Terminal's or SQL's records, honoring their JSON field names.
When several object values match, object keys are visited in sorted order,
which keeps multi-match lists deterministic.
