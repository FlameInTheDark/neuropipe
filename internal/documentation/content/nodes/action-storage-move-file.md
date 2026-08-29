# Move Storage File

## Purpose

Renames or moves one remote entry inside a registered storage. FTP servers
rename atomically, folders included. S3 has no rename: files move through a
server-side copy followed by a delete, and moving populated folders is
rejected with an explicit error rather than copying them object by object.

## Parameters and results

Pick the **Storage** connection, then set the source **From** and the
destination **To** paths. Both stay inside the same storage connection. The
result reports the original path, the destination path, and whether the
entry **Moved**.
