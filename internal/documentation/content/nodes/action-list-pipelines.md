# List Pipelines

## Purpose
Emit the published pipelines of this workspace as structured data, so a flow
can reason about the catalogue itself — filter it, pick a target for the Run
Pipeline node, or feed names into reports and notifications.

## Outputs
- **Then**: exec continuation after the list is collected.
- **Pipelines**: list of objects with `id`, `name`, `description`, `status`,
  and `publishedRevision`. Draft-only pipelines are not included.
- **Count**: how many entries the list contains.

## Configuration
This node has no configuration; the workspace catalogue is read live on each
execution.

## Example
`Manual Trigger → List Pipelines → JavaScript (pick by name) → Run Pipeline`