# KV List Range

## Purpose

Reads a slice of a list with `LRANGE` without consuming it. The **Start** and
**Stop index** fields (inspector or wired pins) default to the whole list
(0 through -1). Use negative
indices to read from the end — exactly like Redis itself.

## Parameters and results

Indexes are zero-based and inclusive on both ends. The **Items** output is a
list in stored order, and **Count** saves a follow-up length node.
