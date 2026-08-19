# Copy Folder

## Purpose

Recursively copy a folder tree to a target path. Files are copied one by one; subdirectories are recreated. Symlinks are skipped with a warning in the result.

Set **Overwrite** to `false` to skip existing target files instead of replacing them.

## Outputs

- `result.targetPath` — the cleaned target path.
- `result.filesCopied` — number of files copied.
- `result.bytesCopied` — total bytes copied.
- `result.warnings` — list of human-readable warning strings (symlinks skipped, existing files skipped when overwrite is disabled).

## Capabilities

Requires **file-read** and **file-write** capabilities.