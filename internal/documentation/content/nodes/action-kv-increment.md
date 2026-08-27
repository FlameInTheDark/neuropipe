# KV Increment

## Purpose

Increments a counter and returns the new value. Integer mode runs `INCRBY`
with the wired **Delta** (default 1); float mode runs `INCRBYFLOAT` for
statistics that need fractional precision. Counters that do not exist start
at zero, which makes the node safe on first use.

## Parameters and results

The counter key and the **Delta** work like every other KV node: inspector
value or wired pin (delta defaults to 1).
The **New value** output is an integer in integer mode and a float in float
mode, matching the pin type of the output.
