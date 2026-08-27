# KV Get

## Purpose

Reads one string value from a registered key/value database (Redis, Valkey,
KeyDB, or Dragonfly). The node runs `GET` on the selected connection and
returns the value together with a `Found` flag, so a missing key is a normal
outcome rather than an error. Connect the **Key** pin to compute key names at
run time, for example by combining an LLM extraction with a key template.

## Parameters and results

Choose the KV database, then set the key either in the inspector or through
the **Key** input pin. The outputs are the stored **Value** (empty when the
key is missing) and **Found**, which distinguishes an empty string from an
absent key. Values larger than 64 KiB are truncated.
