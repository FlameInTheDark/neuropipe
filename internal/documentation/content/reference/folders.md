# Folder nodes

Folder-specific Blueprint nodes live under the **Folders** category. They cover inspecting, copying, moving, deleting, renaming, and listing directories.

## Listing and inspecting

- **List Directory** — list the immediate children of a folder.
- **Folder Exists** — return whether a path is a directory.
- **Wait For Folder** — poll a path until a directory appears or the timeout elapses.

## Copying and moving

- **Copy Folder** — recursively copy a folder tree. Symlinks are skipped with a warning.
- **Move Folder** — atomic rename; falls back to walk-copy-then-delete on cross-device moves.

## Deleting and renaming

- **Delete Folder** — recursively remove a folder tree. Optionally succeed when already missing.
- **Rename Folder** — rename within the same parent directory. The new name must be a single path segment.
