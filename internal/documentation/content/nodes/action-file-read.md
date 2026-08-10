# Read File

## Purpose
Reads an approved local file without changing its data. Select **Bytes** or
**Text** in the inspector to declare the single result pin. Text is accepted
only when the file contents are valid UTF-8; otherwise select Bytes.

## Example
`File Watch → Read File → Parse JSON → For Each Loop`.

Use **Base64 Encode** to turn a selected Bytes or Text value into a selected
Base64 representation. Use **Base64 Decode** to restore it. Neither node
silently converts data.
