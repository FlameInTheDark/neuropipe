# JavaScript

## Purpose

Runs a small, synchronous JavaScript action in the current Blueprint execution.
Use **Edit code** to declare typed input and output pins and write the program.
The program must return one object whose keys exactly match every configured
output ID. No value is converted implicitly: output values must satisfy the
declared `TypeSpec`.

## Example

Declare a required `name` input and a `message` output, both as **Text** in the
type picker. Then write:

```js
return { message: `Hello, ${name}!` };
```

The same input is available through `inputs.name`. This makes it convenient to
read inputs whose IDs are chosen dynamically by a graph author.

## Read inputs

Input IDs become local JavaScript variables, so the configured `name` pin is
available as `name`. Every pin is also in the `inputs` object, including for
dynamic access:

```js
const direct = name;
const property = inputs.name;
const dynamic = inputs["name"];
const label = inputs.optionalLabel ?? "Untitled";
return { message: `${direct}: ${label}` };
```

Pin IDs must be valid JavaScript identifiers, such as `userName`, `count`, or
`filePath`. Names such as `first-name`, `inputs`, and `np` are rejected. A
missing required input fails the node safely; use `??` only for optional pins.

## System API and safety

`np` is a deliberately small local API. It provides run variables, Base64 and
SHA-256 helpers, pipeline/function/trigger/execution summaries, reports, chat
helpers, and desktop notifications. File and network operations are disabled
until the node explicitly enables the matching capability in its editor:

```js
const text = np.files.readText("C:/work/todo.txt"); // requires Read files
const result = np.http.request({ url: "https://example.test/status" }); // requires Network requests
return { message: `${result.status}: ${text}` };
```

The runtime exposes no Go objects, filesystem/process handles, secrets, or
host globals. It has a bounded execution time and memory/call-stack limits.
Read [JavaScript node](../guides/javascript-node.md) for the complete API and
more examples.
