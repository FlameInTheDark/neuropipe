# JavaScript node

The **JavaScript** node is for a compact, deterministic transformation or
local orchestration step that does not justify a new first-party node. It uses
the pure-Go Goja interpreter; it is synchronous, bounded, and does not expose
the application’s Go objects directly.

## Create a typed step

1. Add **Code → JavaScript** after an exec pin and select **Edit code**.
2. Add inputs and outputs. Pin IDs must be valid JavaScript identifiers. They
   become local variables; the object `inputs` also contains all inputs.
3. Choose each pin’s type with the visual picker. It supports primitives,
   lists with typed items, text-keyed maps with typed values, and structural
   objects with named required or optional fields. Neuropipe stores the
   resulting `TypeSpec` for you; there is no JSON contract to edit.

4. Return an object containing every configured output and no additional
   outputs. The runtime validates every nested value. `any` can receive a
   concrete value, but it cannot be narrowed without **Type Assert**.

```js
const titles = tasks.filter((task) => !task.done).map((task) => task.title);
return { titles, count: titles.length };
```

Declare `titles` as `list[string]` and `count` as `int`. A number with a
fraction cannot be returned to an `int` output; use a deliberate conversion in
the script and return a finite safe integer.

## Read input pins in code

Every configured input ID becomes a local JavaScript variable. An input named `name` can therefore be read as `name`; all inputs are also available from the `inputs` object, which is useful when accessing a dynamic key:

```js
const greeting = `Hello, ${name}!`;
const sameName = inputs.name;
const selected = inputs["name"];
return { greeting };
```

Use a valid JavaScript identifier for every pin ID, such as `userName`, `count`, or `filePath`. IDs such as `first-name`, `inputs`, and `np` are rejected. A missing required input stops the node safely. For an optional input, use a deliberate fallback such as `const label = inputs.label ?? "Untitled"`.

## `np` API

All APIs are synchronous and may throw a normal JavaScript error. Catch one
only when the graph can recover deliberately; otherwise let it fail the node.

| API | Use |
| --- | --- |
| `np.context` | Current node ID and, when available, pipeline/execution IDs. |
| `np.uuid()`, `np.assert(value, message)`, `np.fail(message)` | Create an ID or stop safely with an intentional message. |
| `np.variables.get/has/set/delete(name)` | Read and write values scoped to this single run. |
| `np.base64.encodeText/decodeText`, `encodeBytes/decodeBytes` | Explicit text/byte Base64 operations. |
| `np.hash.sha256(textOrBytes)` | Return a SHA-256 hex digest. |
| `np.getPipelines()`, `np.pipelines.list/get`, `np.functions.list`, `np.triggers.list`, `np.executions.list` | Read bounded application summaries. |
| `np.reports.list/get/create` | Browse or create a local Markdown report. `create` takes `{ title, markdown, tags? }`. |
| `np.chat.history/reply/setStatus` | Work with an existing chat run. |
| `np.files.list/readBytes/readText/writeBytes/writeText` | Local file access; requires **Read files** and/or **Write files** on this node. |
| `np.http.request({ url, method?, headers?, body? })` | A bounded HTTP request; requires **Network requests**. Returns `status`, `headers`, and text `body`. |
| `np.notify(title, message)` | Send a desktop notification. |

For example, create a report from declared `title` and `markdown` inputs:

```js
const report = np.reports.create({ title, markdown, tags: ["generated"] });
np.notify("Report created", report.title);
return { reportId: report.id };
```

## Capabilities and trust

Checking an access toggle adds that capability to the node’s resolved contract.
It is collected during publish, shown in the normal trust request, and enforced
again at runtime. A script cannot gain file or network access by constructing
paths, importing modules, or using an unlisted API. Keep access narrow: use
the built-in file and HTTP nodes when their explicit visual contracts fit.

## Limits and errors

Code is syntax-checked before it is saved and compiled again by the backend.
Execution is limited to a short bounded slice of time and a bounded stack. It
cannot `await`, import packages, start a process, access a secret, or retain a
Go value. Invalid input/output shapes, unsupported bytes, malformed Base64,
missing required outputs, capability violations, and interpreter failures stop
the current execution path with a safe error in the run log.

Avoid putting sensitive values into scripts, error messages, reports, or
notifications. Use a first-party node and a saved secret reference for
credentials instead.
