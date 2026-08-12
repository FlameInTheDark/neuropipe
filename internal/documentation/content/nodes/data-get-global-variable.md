# Get Global Variable

## Purpose
Reads a workspace-scoped variable shared across every pipeline and run. A read
before any write returns the variable's declared default value.

The variable is selected from a picklist of declared names managed on the
**Variables** screen. Reads are safe across concurrently running pipelines.

## Example
`Get Global Variable (visits) → Math Add → Set Global Variable` to accumulate
a counter across runs.
