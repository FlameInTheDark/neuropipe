# KV TTL

## Purpose

Reads a key's remaining time to live in seconds with `TTL`. Use it to decide
whether a cached value needs refreshing soon, or to audit which keys are
about to expire.

## Parameters and results

The **TTL seconds** output follows the Redis convention: a positive number is
the remaining lifetime, `-1` means the key has no expiry, and `-2` means the
key no longer exists.
