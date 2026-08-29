# Presign URL for Storage

## Purpose

Generates a temporary signed URL for one object in an **S3** storage — the
link works without credentials until it expires. Pick the HTTP method the URL
is signed for: **GET** (download), **PUT** (upload), **HEAD** (metadata), or
**DELETE** (remove). Signing happens locally against the connection's stored
credentials; nothing is sent to the server. FTP storages are rejected with an
explicit error because FTP has no presigned-URL equivalent.

## Parameters and results

Pick the **Storage** connection, set the object **Path**, and choose the
**Method**. **Expires** accepts seconds (`3600`), Go durations (`15m`,
`1h30m`), or day units (`7d`); blank means one hour, and the SigV4 window
allows 1 second to 7 days. **Required headers** signs those headers into the
URL — key=value lines like `Content-Type=image/png` — and whoever uses the
link must send exactly those headers or the signature fails. **Query
parameters** become signed parameters, typically the S3 response overrides
like `response-content-disposition=attachment; filename="chart.png"`.

The **URL** output pin carries the link directly; the **Result** object also
reports the `method`, `expiresInSeconds`, `expiresAt` (UTC), and the
canonicalized required `headers` map. Example: Upload File → Presign URL
(PUT, 15m) → HTTP Request.
