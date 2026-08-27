# KV Set Members

## Purpose

Reads every member of a set with `SMEMBERS`. The output is a plain list that
feeds For Each loops, filters, and Build Object nodes directly.

## Parameters and results

A missing key yields an empty list with count zero rather than an error.
Very large sets are capped at 500 members with a truncated flag on the
service boundary, matching the SQL node's row cap.
