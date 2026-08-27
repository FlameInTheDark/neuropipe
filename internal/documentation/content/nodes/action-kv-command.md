# KV Command

## Purpose

Runs any Redis command against a registered KV database. It is the
key/value counterpart of the SQL node: curated nodes cover the common
structures, and this node guarantees the full command surface — streams,
geospatial queries, bitmaps, and anything else — is reachable from a
pipeline.

## Parameters and results

Configure the command in the inspector (or wire the **Command** pin to
compute it at run time) and declare typed **Arguments** in the visual
argument builder — name, label, type, and required per row — exactly like SQL
parameters: each argument becomes one typed input pin, and values are
converted to their Redis string form when the node runs. List and object
arguments are JSON-encoded so complex values stay lossless.

Administratively destructive commands (`FLUSHALL`, `CONFIG`, `SHUTDOWN`, and
the rest of the denylist) are rejected unless **Allow dangerous commands**
is enabled. The reply is normalised to JSON-safe values and exposed as
**Value**, with **Value (text)** for string rendering and **Is nil** for
Redis nil replies. Replies are capped at 500 collection elements and 64 KiB
per string.
