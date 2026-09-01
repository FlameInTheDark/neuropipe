# Unzip Files

## Purpose

Extract a `.zip` archive into a target directory. The **Overwrite** input controls whether existing files in the target are replaced.

Zip-slip paths are rejected explicitly and identically on every platform: entries that are rooted (a leading `/` or `\`), start with a Windows drive-letter prefix (like `C:`), or contain any `..` component are refused before anything is written. Empty directory entries are recreated. Symlinks inside the archive are skipped.

## Outputs

- `result.extractedFiles` — list of relative paths extracted.
- `result.entryCount` — count of extracted file entries.

## Capabilities

Requires **file-read** and **file-write** capabilities.