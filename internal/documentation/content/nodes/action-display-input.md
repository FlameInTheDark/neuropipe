# Display Input Dialog

## Purpose
Show a styled dialog window with a title, message, labelled input field, and
Continue/Cancel buttons. Execution blocks until the user responds. Continue
emits the typed value on the Value pin and routes from the Continue pin;
Cancel routes from the Canceled pin and emits nil on the Value pin.

## Configuration
- **Title**: text shown in the dialog title bar.
- **Message**: text shown in the dialog body, typically a prompt for the
  expected input.
- **Field label**: label displayed next to the input field.
- **Input type**: `text` accepts any string, `number` parses the input as a
  float and fails the run if it is invalid. The Value output pin follows
  this type so the editor highlights incompatible connections.

## Example
`Button Trigger → Display Input Dialog (Input type: number) →
  Continue → Math: Add (use the Value pin),
  Canceled → Desktop Notification (cancelled message)`.
The Value pin is `nil` when the user cancels, so downstream nodes can detect
the cancellation through a `Get Type` or `Equals` check.
