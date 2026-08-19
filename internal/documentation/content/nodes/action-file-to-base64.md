# File To Base64

## Purpose

Read a local file and encode its contents as a standard Base64 string. Use this node when you want to embed binary file content into a text-only transport (for example an HTTP body, an LLM prompt, or a JSON field).

For the reverse operation, use **Base64 To File**. For encoding connected bytes directly (no file read), use **Bytes To Base64**.

## Outputs

- `result` — the Base64-encoded file content.

## Capabilities

Requires **file-read** capability.