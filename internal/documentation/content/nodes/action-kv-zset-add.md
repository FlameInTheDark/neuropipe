# KV Sorted Set Add

## Purpose

Adds or updates scored members in a sorted set with `ZADD`. Sorted sets back
leaderboards, priority queues, and delay schedules because members are
ordered by score at write time.

## Parameters and results

Entries are `{ "member": ..., "score": ... }` objects supplied through the
inspector's member/score editor or as a wired **Entries** pin. Updating an
existing member's score does not count toward **New members**; only genuinely
new members do.
