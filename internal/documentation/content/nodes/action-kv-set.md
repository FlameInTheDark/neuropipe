# KV Set

## Purpose

Writes one string value with `SET` and an optional expiry in seconds. The
write condition can require the key to be new (`NX`) or existing (`XX`), which
turns the node into a lock primitive or a guarded update. Enable **Return
previous value** to receive the value that was replaced, if any.

## Parameters and results

Key, value, and TTL can all be typed directly in the inspector or wired as
pins. The **TTL seconds** value is optional; zero writes without an expiry. When the
condition rejects the write, **Set** reports false without failing the node.
With the previous-value option enabled, the reply is exposed on the
**Previous** output instead.
