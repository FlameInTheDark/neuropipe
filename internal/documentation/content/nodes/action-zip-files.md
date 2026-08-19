# Zip Files

## Purpose

Compress one or more files or folders into a local `.zip` archive. Pass a `;`-separated list of source paths on the **Paths** input, the destination folder on **Target directory**, and the desired archive name on **Archive name**.

Folders are walked recursively; their relative paths are preserved inside the archive. Symlinks are skipped silently. Files with duplicate names get an ` (N)` suffix to avoid overwrites inside the zip.

## Outputs

- `result.archivePath` — absolute path of the written archive.
- `result.entryCount` — number of file entries written.
- `result.bytesWritten` — total uncompressed bytes streamed into the archive.

## Capabilities

Requires **file-read** and **file-write** capabilities.