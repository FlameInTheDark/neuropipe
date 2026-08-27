# KV Scan

## Purpose

Pages through keys with cursor-based `SCAN`, optionally filtered by a glob
pattern and a key type. Unlike `KEYS`, scanning never blocks the server, so
it is safe on production-sized keyspaces. On embedded SugarDB connections,
which have no `SCAN` command, the same cursor contract is served transparently
over `KEYS` with offset-based pages.

## Parameters and results

Start from cursor 0, pass the returned **Next cursor** back into the **Cursor**
pin on the next call, and stop when **Done** is true — the classic
while-loop pattern with a Flow While node. **Keys** is the page of matching
key names. The page size is a hint between 1 and 500; servers may return
smaller or slightly larger pages, and an empty page with a non-zero cursor is
normal.
