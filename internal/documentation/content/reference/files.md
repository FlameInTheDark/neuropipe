# File nodes

Neuropipe provides a set of file-specific Blueprint nodes for inspecting, copying, moving, deleting, renaming, and converting local files. All nodes live under the **Files** category in the library panel.

## Inspecting files

- **File Exists** — returns whether a path is a regular file.
- **Wait For File** — polls a path until a file appears or the timeout elapses.

## Copying and moving

- **Copy File** — copy a file to a target path, creating the parent directory if missing.
- **Move File** — atomic rename with a cross-device copy+delete fallback.

## Deleting and renaming

- **Delete File** — remove a regular file; optionally succeed when already missing.
- **Rename File** — rename within the same directory. The new name must be a single path segment.

## Encoding helpers

- **File To Base64** — read a file and return its contents as Base64 text.
- **Base64 To File** — decode a Base64 string into a file. Auto-detects standard and URL-safe variants.

For folder-specific operations, see **Folder nodes**. For zipping and unzipping, see **Archive nodes**.
