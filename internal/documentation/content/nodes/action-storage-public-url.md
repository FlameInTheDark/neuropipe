# Get Public URL from Storage

## Purpose

Builds the public address of one file or folder in any registered storage
without a network round-trip, so URLs can be produced before a file even
exists. When the connection has a **Public URL base** configured (Storages →
Edit), the URL joins that base — a CDN in front of a bucket, or a web server
serving the FTP tree — and works for both S3 and FTP. Otherwise S3 storages
return the direct object address (`https://bucket.s3.region.amazonaws.com/key`
for AWS, path style for custom endpoints), and FTP storages fall back to a
best-effort `ftp(s)://host/base/path` protocol URL that includes the base
folder but never embeds credentials.

## Parameters and results

Pick the **Storage** connection and set the **Path** (empty means the storage
root). The **URL** output pin carries the address directly; the **Result**
object also reports the `kind` of URL: `public-base`, `s3`, or `ftp`. Note
that a public URL only succeeds publicly when the object is actually readable
— presigned links (Presign URL node) are the way to share private objects.
Example: Upload File → Public URL → Discord Send Message.
