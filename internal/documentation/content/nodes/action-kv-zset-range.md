# KV Sorted Set Range

## Purpose

Reads a rank-ordered slice of a sorted set with `ZRANGE` or `ZREVRANGE`,
always including scores. Highest-score-first order powers leaderboards; the
ascending order feeds priority processing.

## Parameters and results

**Start** and **Stop rank** (inspector fields or wired pins) default to the
whole set (0 through -1) and are inclusive, like Redis. The **Entries** output is a list of
`{ "member": ..., "score": ... }` objects ready for display or further
filtering.
