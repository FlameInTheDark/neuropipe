# KV Hash Get

## Purpose

Reads hash data with `HGET` or `HGETALL`. When the **Field** input is set the
node reads one field and returns its **Value**; when it is empty the node
reads the whole hash and returns every entry on the **Fields** object output.

## Parameters and results

A missing field or empty hash reports `Found` false instead of failing, so
lookups can fall through to a default path. The whole-hash output is an
object, which connects directly to Get Field or JavaScript nodes for further
shaping.
