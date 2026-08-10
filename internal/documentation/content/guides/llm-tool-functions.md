# LLM tool functions

An LLM tool function is a reusable, published Blueprint function that an **Agent** or **Coding Agent** can call through its **Tools** pin. It is not an exec node: its single Tool output only declares availability to the model. The host validates the call, runs the function, and sends the typed result back to the model.

Use a tool for a narrow capability with a clear input and a useful result. Good tools answer one question or perform one deliberate action. Keep broad planning and decisions in the Agent instructions.

## Build the contract first

1. In **Functions**, select **New function → LLM tool**.
2. Write a description that says *when* the model should call it and what it accomplishes. For example: “Look up the current forecast for a named city. Use this only when the user asks for weather.”
3. Add public inputs and outputs in the function editor. Every pin needs **Model guidance**. State the meaning, constraints, and a short example.
4. Choose concrete types and mark only genuinely necessary inputs as required. Publish after the graph has a reachable **Function Entry → Function Return** path.

The publish check rejects an ungrounded tool: it needs a function description, at least one output, guidance for every public pin, concrete types, and unique model argument names. A tool can have no inputs, but it must still return a described result.

## Example: weather lookup

Create a tool named **Get city forecast**.

| Part | Contract |
| --- | --- |
| Description | “Look up the current forecast for a city. Use when a user asks about weather.” |
| Input `city` | Text, required. Guidance: “City and country to look up, for example `Yekaterinburg, RU`.” |
| Output `forecast` | Text. Guidance: “A concise current forecast with conditions and temperature.” |

Inside the function, wire control flow as follows:

```text
Function Entry ──exec──> HTTP Request ──exec──> Function Return
      city ──data──> HTTP request mapping ──data──> forecast
```

Publish the function. In a pipeline, add an **Agent**, add the published **Get city forecast** tool node, then connect its **Tool** output to the Agent’s **Tools** input. Tool connections are unlimited: connect several independently useful tools to the same Agent.

Use focused Agent instructions, for example:

> Answer the user’s weather question. When a current forecast is needed, call the connected Get city forecast tool with the city and country. Do not invent a forecast. Summarize the tool result in one short answer.

The model sees a safe provider name derived from the function name, the function description, input names, JSON schemas, required inputs, input guidance, and a description of the returned fields. It never receives tools that are not wired to the Agent.

## Types at the model boundary

Tool arguments are JSON, but Neuropipe decodes them into the function’s exact Go-inspired `TypeSpec` before execution.

| Function type | Model argument |
| --- | --- |
| Text, Boolean | JSON string or boolean |
| Integer, Float | JSON number; an integer must have no fraction |
| Bytes | Base64 string, decoded to `[]byte` before the function runs |
| List | JSON array whose items satisfy the declared element type |
| Map | JSON object with text keys and declared value type |
| Anonymous record | JSON object with the declared fields; unknown fields are rejected |

`any`, named Go records, and maps with non-text keys are not valid public tool contracts because a model cannot describe or supply them safely through JSON. Use an anonymous structural record for an object-shaped argument instead. There is no text-to-number, integer-to-float, or byte-to-text conversion at this boundary.

For example, a binary upload tool can accept `content` as **Bytes**. Its guidance should say “Base64-encoded UTF-8 document bytes.” The model sends Base64; the function receives a byte slice. If the tool needs text, create an explicit conversion inside the graph rather than changing the contract silently.

## Calls, errors, and trust

The Agent makes at most its configured number of turns. Each requested tool call is checked against the published signature before the function starts. Unknown arguments, missing required values, wrong types, malformed Base64, or unknown record fields are returned to the model as a safe contract error, so it can correct the call in another turn. Internal execution failures are not exposed with local paths, secrets, or payloads.

The tool function executes with the capabilities declared by the nodes inside it. Publishing or running a pipeline still requires the normal local trust and capability approvals. Do not put credentials in the function description, pin guidance, Agent instructions, or tool results; retrieve them through the approved node configuration instead.

## Design checklist

- Give the tool one specific verb: look up, create, check, send, or update.
- Make the description explain when to use it and when not to.
- Name inputs by their meaning, not their UI label: `city`, `repository`, `recipient`, `query`.
- Make outputs useful to the next model turn; return a status and concise details for actions.
- Keep side effects explicit and require the least capability possible.
- Publish again after changing the public signature, then repair any affected Agent wires.
