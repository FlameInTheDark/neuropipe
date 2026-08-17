# Display Message

## Purpose
Show a native dialog window with a title, message, and OK button. Execution
blocks until the user dismisses the dialog, then continues from the Then pin.

## Configuration
- **Title**: text shown in the dialog title bar.
- **Message**: text shown in the dialog body. Multi-line text is supported.

## Example
`Button Trigger → Display Message (Title: Done, Message: Pipeline finished)`.
Connect `Format Text` to the **Message** pin to display dynamic values
computed earlier in the run.
