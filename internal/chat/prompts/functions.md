# Neuropipe function authoring

A custom function packages reusable graph logic. It is a Blueprint graph with
declared boundary pins, publishable like a pipeline and callable from other
graphs as a `function:<functionId>` node.

## JSON shape

```json
{
  "name": "Fetch and summarize",
  "description": "Downloads a URL and returns a short summary.",
  "mode": "impure",
  "inputs":  [],
  "outputs": [],
  "draftDefinition": { ...same graph JSON as pipelines... }
}
```

- `mode`: `"pure"` (deterministic, no side effects) or `"impure"`
  (performs actions: HTTP, files, notifications, running pipelines).
  LLM tool functions must be `"impure"`.
- `inputs` / `outputs`: boundary pins.
  Each pin: `{"id": "url", "name": "URL", "dataType": "text",
  "required": true, "description": "...", "type": {TypeSpec, optional}}`.
  `dataType` is one of text/number/boolean/array/map/object/any.

## Boundary nodes by mode

Pure functions:

- exactly one `function:input` node (its OUTPUTS are the declared inputs)
- exactly one `function:output` node (its INPUTS are the declared outputs)
- no `function:entry` / `function:return`
- only pure nodes inside (no triggers, no impure actions)

Impure functions:

- exactly one `function:entry` (OUTPUTS = inputs) whose exec output starts
  the flow; a `function:return` (INPUTS = outputs) reachable from it
- no `function:input` / `function:output`
- may contain impure nodes; still no event triggers

Tool functions (`kind: "tool"`, set when saving):

- must be `"impure"`. The entry's outputs become the agent-visible
  parameters; the return's inputs become the reply payload. Keep names
  snake_case-friendly and descriptions precise — they are shown to models.

## Rules enforced on save/publish

- unique pin ids; required TypeSpec validity; no recursion (a function may
  not call itself directly or transitively); entry→return reachability for
  impure mode; node types must exist in the catalog.

## Workflow

1. Draft the inner graph first (see pipeline authoring guide).
2. Wrap it with the correct boundary nodes for the chosen mode.
3. `save_function_draft` — fix validation errors, re-save.
4. Offer `publish_function` so graphs can reference `function:<id>`.
