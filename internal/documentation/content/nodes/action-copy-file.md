# Copy File

## Purpose

Copy a regular file to a target path. Creates the parent directory of the target if missing. Set **Overwrite** to `false` to fail instead of overwriting an existing target.

## Outputs

- `result.targetPath` — the cleaned target path.
- `result.bytesCopied` — bytes written.

## Capabilities

Requires **file-read** and **file-write** capabilities.