# Archive nodes

The **Archive** category contains two nodes for working with local `.zip` archives.

## Zip Files

Compress one or more files or folders into a `.zip` archive. The **Paths** input accepts a `;`-separated list (e.g. `C:\Work\file1.txt;C:\Work\folder`). Folder trees are walked recursively, preserving relative paths inside the archive. Symlinks are skipped silently.

## Unzip Files

Extract a `.zip` archive into a target directory. The **Overwrite** input controls whether existing files are replaced. Zip-slip paths (entries whose normalized target lands outside the target directory) are rejected explicitly. Symlinks inside the archive are skipped.
