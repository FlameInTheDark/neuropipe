# KV Set Remove

## Purpose

Removes members from a set with `SREM` and reports how many were actually
present. It is the cleanup counterpart to KV Set Add — for example, dropping
a user from an active-viewers set when their session ends.

## Parameters and results

Members arrive through the inspector's list editor or a wired pin. Removing a member that is not
in the set simply does not count toward **Removed members**.
