# KV Expire

## Purpose

Sets or removes a key's expiry in seconds. Expire mode runs `EXPIRE` with the
wired **TTL seconds**; persist mode runs `PERSIST` and removes the expiry
entirely. Use it to implement sliding sessions or to bound how long a
computed cache entry survives.

## Parameters and results

TTL seconds (inspector field or wired pin) must be greater than zero in
expire mode. The **Applied** output
is false when the key did not exist at the moment the command ran.
