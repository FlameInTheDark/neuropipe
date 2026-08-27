# KV Sorted Set Remove

## Purpose

Removes members from a sorted set with `ZREM`. Use it to retire leaderboard
entries, cancel delayed jobs, or drop expired priorities.

## Parameters and results

Members arrive through the inspector's list editor or a wired pin, and **Removed members** counts
only the members that existed in the sorted set.
