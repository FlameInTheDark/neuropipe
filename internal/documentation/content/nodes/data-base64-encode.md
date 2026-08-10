# Base64 Encode

## Purpose

Explicitly encodes one selected **Bytes** or **Text** input as Base64. Select
the input and output representations independently; a Bytes output is the
Base64 byte sequence, while Text is its UTF-8 string form.

## Example

`Read File (Bytes) → Base64 Encode → HTTP Request`.

Changing a selector changes the exact wire type. It does not reinterpret,
parse, or coerce a connected value.
