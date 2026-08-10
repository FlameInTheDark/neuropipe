# Write File

## Purpose
Writes a selected **Text** or **Bytes** value to an approved local path. The
Content type selector changes the exact input-pin contract; Bytes must arrive
through a connected Bytes wire rather than being parsed from the text editor.

## Example
`Read File (Bytes) → Write File (Bytes)` after a visible exec pulse reaches
the write node.
