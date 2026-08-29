# Download File from Storage

## Purpose

Streams one remote file from a registered S3 or FTP storage to a local
path. Bytes flow straight to disk, so large objects download without
buffering the whole payload. Missing parent directories are created
automatically.

## Parameters and results

Pick the **Storage** connection, then set the remote path and the local
destination. When the local path ends with a path separator, the remote file
name is appended. The result reports the written local **Path**, the remote
file **Name**, and the downloaded byte count.
