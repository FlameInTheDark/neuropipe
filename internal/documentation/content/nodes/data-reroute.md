# Data Reroute

## Purpose
Reorganises a data wire without changing its value. The reroute acts like a pin on a node: its input accepts exactly one wire, and its output can fan out to several targets.

## Typed pass-through
The output pin mirrors whatever feeds the input: connect a Text wire and the output becomes Text, connect a Number wire and it becomes Number — the pin colour, type checks, and downstream wire colours all follow the connected source. Disconnecting the input returns the pin to Any until a new wire arrives.

Insert one on an existing wire through the wire's context menu (**Insert reroute**) or add it from the palette; drag from its output pin to create each additional connection.

## Example
`LLM Prompt.Result → Data Reroute → Create Report`, with a second wire from the same reroute feeding Get Field.
