# Upload Data to Storage

## Purpose

Writes raw bytes straight into a registered S3 or FTP storage without a
local round-trip. The **Data** pin matches the Draw Image node's image
output, so a rendered picture wires directly; HTTP download bodies and
base64 text (including `data:` URLs) work too. In-memory uploads are capped
at 512 MiB per node run.

## Parameters and results

Pick the **Storage** connection, wire the **Data** input, and set the remote
path including the file name. The optional content type is sent to S3 (blank
sniffs the payload); FTP ignores it. The result reports the stored **Key**,
the written **Size** in bytes, and the **Driver** that handled the write.
