# Set Global Variable

## Purpose
Writes a workspace-scoped variable shared across every pipeline and run. The
value is held in memory immediately and persisted to the local database at
most once per second, so it survives an application restart.

The node performs one of three operations, selected in the inspector:

- **Set** overwrites the value, validating it against the declared data type.
- **Increment** atomically adds a number, preventing lost updates when two
  pipelines run at once.
- **Append** atomically appends an item to a list.

## Example
`Trigger Cron → Set Global Variable (lastRun, operation: set)` to record the
last time a maintenance pipeline ran.
