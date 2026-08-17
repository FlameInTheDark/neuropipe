# HTML Extract

## Purpose
Extracts exact values from an HTML document with CSS selectors, the way n8n's HTML node does. Every configured query creates its own typed output pin.

## Configuration

Connect an HTML document to the **HTML** input, then add extractions in the node inspector. Each extraction defines:

- **Pin name** for the output wire.
- **CSS selector** to match elements, for example `h1.title` or `ul li a`.
- **Return value**: **Text** (the element's text content), **HTML** (the element's rendered markup), or **Attribute** (an attribute such as `href`; the attribute name field appears when selected).
- **Return all matches**: off returns the first match as Text; on returns every match as a list of Text values.

Outputs use only the default data types (Text or List), so results connect directly to Format Text, For Each, Join, Regex, and every other node.

A selector with no matches is not an error: the output is empty text, or an empty list when returning all matches. An invalid CSS selector or a duplicate pin name fails validation before the pipeline runs.

## Example
`HTTP Request → HTML Extract (selector `a.product`, Return value: Attribute `href`, Return all matches) → For Each`.

Combine with the HTTP Request toggles **Remove scripts** and **Remove styles** to pass the extractor clean markup.
