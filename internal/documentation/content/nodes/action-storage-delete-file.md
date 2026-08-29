# Delete from Storage

## Purpose

Deletes one remote file, or a folder with everything inside it, from a
registered S3 or FTP storage. Folders require the recursive option so a
single mis-set path cannot wipe a tree by accident.

## Parameters and results

Pick the **Storage** connection and set the remote path. Enable **Delete
folder contents recursively** to remove folders; without it, deleting a
folder fails with an explicit error. The result reports whether anything was
**Deleted** and the **Count** of removed entries — recursive deletes count
every child object.
