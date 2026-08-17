# Form

## Purpose
Show a full-screen form modal built from a grid-based layout. Each input
field and dropdown becomes a typed output data pin. The user fills the form
and clicks Submit (routes from the Submit exec pin) or Cancel (routes from
the Canceled exec pin).

## Configuration
- **Title**: text shown in the form modal's title bar.
- **Message**: optional description shown below the title.
- **Form layout**: visual grid builder where you place text panels, input
  fields, and dropdown selectors. Each item can span 1–4 columns. Inputs
  can be text or number. Dropdowns have labeled options (display text +
  return value) or value-only options.

## Example
`Button Trigger → Form → Submit → Display Message (showing the submitted values)`
