# Switch

## Purpose

Switch is an impure Blueprint flow-control node. An Exec pulse resolves its
**Value** data pin, tests the configured cases in order, and follows exactly
one exec output: the first matching case or **Default**.

## Pins

- **Exec** — starts the comparison.
- **Value** — any data value to compare. The stable pin ID remains compatible
  with existing graphs.
- **Configured case outputs** — one exec pin per case, labelled with its Pin
  name.
- **Default** — runs when no case matches. Leaving it disconnected ends that
  execution branch normally.

## Configuration

Choose one comparator for the whole node. Each ordered case has a typed literal
**Value** and an editable **Pin name**. Pin IDs are generated and remain stable,
so changing a name does not break existing wires. Move cases up or down to
change priority.

| Comparator | Supported case value |
| --- | --- |
| Equals, Not equals | Text, Number, Boolean |
| Contains, Starts with, Ends with | Text |
| Greater than, Greater than or equal, Less than, Less than or equal | Number |

Neuropipe never converts text to numbers or Booleans. A text comparator needs
a text input; a numeric comparator needs a number input. Invalid case settings
are rejected before publishing.

## Execution result

The node run result records the input value, comparator, and matched case ID
and name. It does not duplicate the result at multiple nesting levels.

## Example

Connect `Get Field priority` to **Value**. Use the `Contains` comparator with
an `urgent` case leading to a desktop notification and a `review` case leading
to Create Report. Connect **Default** to the normal workflow. If the input is
`urgent-review`, the first matching configured case runs.
