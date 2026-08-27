# KV List Push

## Purpose

Pushes values onto a list with `RPUSH` (tail, the default) or `LPUSH` (head)
and returns the new list length. Lists are the natural queue primitive: pair
a producer pipeline that pushes work with a consumer that pops it.

## Parameters and results

Values arrive through the inspector's list editor or a wired **Values** pin;
scalar pins wired to a list input are converted with the same rules as the
generic KV Command node. **List length** reflects the list after the push.
