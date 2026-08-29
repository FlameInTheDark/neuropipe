# List Storage Files

## Purpose

Lists the direct children of one folder in a registered S3 or FTP storage —
the same listing the Storages browser shows. An empty path lists the root.
On S3, folders are common prefixes; on FTP they are directory entries.

## Parameters and results

Pick the **Storage** connection and set the folder path (empty for the
root). The **Entries** output is a list of objects with `name`, `path`,
`isDir`, `size`, and `modified` fields; **Count** is the number of returned
entries. The listing is never recursive — descend by feeding an entry's
`path` back into the node.
