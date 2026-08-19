# Unzip Files

## Purpose

Extract a `.zip` archive into a target directory. The **Overwrite** input controls whether existing files in the target are replaced.

Zip-slip paths (entries whose normalized target lands outside the target directory) are rejected explicitly. Empty directory entries are recreated. Symlinks inside the archive are skipped.

## Outputs

- `result.extractedFiles` — list of relative paths extracted.
- `result.entryCount` — count of extracted file entries.

## Capabilities

Requires **file-read** and **file-write** capabilities.