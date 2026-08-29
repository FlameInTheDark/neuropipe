# Create Storage Folder

## Purpose

Creates one folder in a registered S3 or FTP storage. FTP servers get a
real directory; S3 has no folders, so the node writes a zero-byte object
with a trailing slash — exactly what the major consoles do.

## Parameters and results

Pick the **Storage** connection and set the folder path. The result reports
the created **Path** and whether it was **Created**. On FTP the parent must
already exist; create nested paths one level at a time.
