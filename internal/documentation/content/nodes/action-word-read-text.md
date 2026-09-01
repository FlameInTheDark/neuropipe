# Read Word Text

## Purpose
Extracts the visible text of a .docx document: paragraphs are separated by
newlines, soft line breaks become newlines, tabs are preserved, and tables
are flattened into tab-separated rows. Use it to feed document content
straight into LLM nodes, reports, or text processing — no external
converter needed.

## Example
`Read Word Text (C:\Work\contract.docx)` → `LLM Summarize` →
`Create Report`.
