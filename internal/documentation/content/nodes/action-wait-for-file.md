# Wait For File

## Purpose

Poll a path until a regular file exists at it, or until the timeout elapses. Two execution outputs (`found` and `timeout`) let pipelines branch on the outcome.

The **Timeout (s)** and **Poll (s)** inputs are exposed as pins so upstream nodes can control them dynamically; the inspector fields provide defaults.

## Outputs

- `found` — execution output fired when the file appears before the deadline.
- `timeout` — execution output fired when the deadline is reached.
- `result.found` — boolean indicating the outcome.
- `result.waitedSeconds` — how long the node waited.

## Capabilities

Requires **file-read** capability.