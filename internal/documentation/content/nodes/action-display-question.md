# Display Question

## Purpose
Show a native dialog window with Yes and No buttons. Execution blocks until
the user chooses one, then continues from the matching exec pin so the
graph can branch on the user's decision.

## Configuration
- **Title**: text shown in the dialog title bar.
- **Message**: text shown in the dialog body. Phrase it as a yes/no question.

## Example
`Button Trigger → Display Question (Message: Send the report now?) →
  Yes → HTTP Request,
  No → Desktop Notification (skipped)`.
The Result output reports which button the user pressed (`yes` or `no`).
