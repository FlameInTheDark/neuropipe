# Blueprint exec and data pins

Neuropipe uses two distinct wire types.

## Exec pins

White arrow pins carry control flow. An event starts a pulse. Each impure node receiving that pulse resolves its data inputs, performs its action, records outputs, and then chooses one exec output. A data connection never performs an HTTP request, command, file operation, LLM call, or report by itself.

## Data pins

Coloured circular pins carry typed values: text, number, Boolean, object, list, or Any. A pure node evaluates only when another node requests one of its outputs. Its result is memoized in the active run frame, so connected consumers reuse the same value.

Impure outputs are also reused during the same activation. If a node requests data from an impure source that has not been reached by an exec pulse on the active path, the run stops with an error naming the source pin. Nothing is cached between runs, branches, or loop iterations.

## Example

`For Each Loop` pulses `Branch`. Before Branch chooses True or False, it asks for Condition. The connected pure comparison evaluates once for that loop frame, using the loop item data, then Branch routes execution.

Use literal defaults for simple values and typed data edges for dynamic values. Template strings are not execution wiring.
