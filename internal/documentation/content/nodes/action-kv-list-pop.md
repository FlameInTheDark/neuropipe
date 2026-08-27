# KV List Pop

## Purpose

Pops values off a list with `LPOP` (head, the default) or `RPOP` (tail).
Together with KV List Push this gives pipeline-safe queue semantics: work
items are claimed exactly once by exactly one consumer.

## Parameters and results

The optional **Count** (inspector field or wired pin) pops several values at
once, returned as the
**Values** list; the first popped value is also exposed as **Value** for
single-item flows. Popping from a missing or empty list returns `Found` false
rather than an error.
