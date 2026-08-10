# List Directory

## Purpose

Lists files, folders, and symbolic links in an approved local directory. Each
entry has a name, absolute path, size in bytes, type, creation time when the
operating system supplies it, and last update time.

## Example

`Button Trigger → List Directory → For Each Loop → Type Assert`.

The **Files** pin is a typed list of records. Connect it to a generic loop, or
use **Type Assert** before a node that requires the exact entry structure.
