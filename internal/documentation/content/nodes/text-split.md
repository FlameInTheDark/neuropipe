# Split

Split exact Text at Separator and preserve empty segments. The result is a typed
`list[string]`; no values are parsed or converted.

Example: `a,,b` split on `,` yields three parts, including the empty middle part.
