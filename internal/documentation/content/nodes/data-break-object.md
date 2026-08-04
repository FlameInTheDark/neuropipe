# Break Object

## Purpose

Splits one object into named, typed output pins. It is the complement of
**Build Object**: configure each output once, then wire the individual values
where they are needed.

## Configuration

Connect an object to **Source**. Each **Outputs** row has a stable output ID,
a visible pin name, a dotted **Field path**, and an expected data type. A path
such as `customer.name` reads nested object values. Numeric path components may
read list items, for example `items.0.name`.

When the connected source is a first-party node with documented result fields,
use **Auto-configure** to create output rows for all known fields at once. You
can still rename pins, remove rows, add paths, and narrow their types manually.
The action only changes this node’s local mapping; it never stores run data.

## Produced values

Each configured output becomes a data pin. Values are resolved lazily and
memoized only inside the current run frame. A missing path returns `null`.
If a non-null value does not match the configured type, the requesting path
stops with a clear type error.

## Example

Connect **Run Terminal Command.Result** to **Source**, then select
**Auto-configure**. The node creates `terminal.command` and `terminal.output`
text outputs. Wire **Output** to an LLM Prompt or Create Report Markdown pin.

~~~text
Button Trigger ──► Run Terminal Command
                       Result ──► Break Object (terminal.output)
                                            Output ──► LLM Prompt
~~~
