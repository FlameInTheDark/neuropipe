# Base64 Decode

## Purpose

Explicitly decodes a selected Base64 **Text** or **Bytes** representation. The
output selector declares either original Bytes or UTF-8 Text.

## Example

`Get Field → Base64 Decode → Write File`.

Malformed Base64 stops the active path safely. Choosing Text for binary output
also fails safely; choose Bytes for arbitrary binary data.
