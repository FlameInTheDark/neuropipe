# Upload File to Storage

## Purpose

Uploads a file into a registered S3 or FTP storage from three different
sources, picked on the **Source** dropdown:

- **From disk** — streams one local file straight to the remote server. The
  file is never fully buffered in memory, so multi-gigabyte uploads work the
  same as small ones.
- **From node** — takes raw bytes from another node. The **Data** pin matches
  the Draw Image node's image output, so a rendered picture wires directly;
  HTTP download bodies and base64 text (including `data:` URLs) work too.
- **From base64** — decodes base64 text typed into the inspector or wired into
  the **Base64** pin.

**Auto** (the default) uses whatever is set: the local path first, then node
bytes, then base64 — exactly how graphs saved before the dropdown behaved.

In-memory uploads (From node and From base64) are capped at 512 MiB per node
run; disk uploads stream and are not capped.

## Parameters and results

Pick the **Storage** connection, choose the source, and set the remote path.
When the remote path ends with `/`, the original file name is appended, so
`reports/` keeps names automatically — for From node and From base64 the
remote path should include the file name. The optional content type is sent
to S3 (blank sniffs the payload); FTP ignores it. An explicit source reads
only its own input, so stale values in hidden fields never leak into the
upload. The result reports the stored **Key**, the uploaded **Size** in
bytes, and the **Driver** that handled the transfer.
