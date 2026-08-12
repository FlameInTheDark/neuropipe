# Flow nodes

Flow nodes are impure: an exec pulse is required before their data outputs exist. Data inputs are resolved just before the pulse is handled.

## Branch

**Purpose:** choose True or False. **Pins:** Exec/Condition inputs; True/False exec outputs. **Configure:** none. **Produces:** selected control flow. **Capabilities:** none. **Failure:** condition must resolve to Boolean. **Example:** Equals → Branch Condition; True → Create Report.

## Sequence

**Purpose:** pulse outputs in order. **Pins:** Exec input; Then 0 and Then 1 outputs. **Configure:** none. **Produces:** ordered flow. **Capabilities:** none. **Failure:** a failing earlier branch stops the run. **Example:** Sequence → notification, then report.

## For Each Loop

**Purpose:** process list items. **Pins:** Exec/Array inputs; Loop Body, Completed, Array Element, Array Index outputs. **Configure:** none. **Produces:** one scoped frame per item. **Capabilities:** none. **Failure:** non-list input or cancellation stops safely. **Example:** Query JSON items → For Each → HTTP Request.

## For Loop

**Purpose:** use inclusive numeric bounds. **Pins:** Exec, First Index, Last Index; Loop Body, Completed, Index. **Configure:** none. **Produces:** loop index. **Capabilities:** none. **Failure:** runtime loop bounds prevent unbounded work. **Example:** Constant 0/10 → For Loop → notification.

## While

**Purpose:** repeat while Condition is true. **Pins:** Exec/Condition; Loop Body/Completed. **Configure:** none. **Produces:** flow. **Capabilities:** none. **Failure:** bounded iterations and cancellation avoid endless runs. **Example:** Get Variable retry → While → HTTP Request.

## Switch

**Purpose:** route one exec pulse through the first configured case that matches a data value. **Pins:** Exec and Value inputs; one exec output for every configured case plus Default. **Configure:** choose one comparator for the node, then add ordered cases with a typed literal Value and a visible Pin name. Each case has an internal stable output ID, so renaming a pin keeps its wires connected. **Produces:** one selected exec path and a compact run result containing the input, comparator, and matched case (if any). **Capabilities:** none.

Cases are evaluated from top to bottom. The first true comparison wins; no later case runs. If no case matches, Default runs. An unconnected Default ends that execution branch normally.

**Comparators:** Equals and Not equals accept text, number, or Boolean literals. Contains, Starts with, and Ends with require text. Greater than, Greater than or equal, Less than, and Less than or equal require numbers. Values are strict: Neuropipe does not coerce `"5"` to `5`. Supplying a non-text value to a text comparator or a non-number value to a numeric comparator stops the run with a clear Switch error.

**Failure:** publishing rejects an empty case list, duplicate pin names/IDs, unsupported comparator/value-type combinations, and malformed literals. Deleting a case in the editor confirms removal of its execution wires. **Example:** Get Field `priority` → Switch Value; configure `Contains` with `urgent` → Notify urgently, then `Contains` with `review` → Create review report, leaving Default → Normal notification.

## Do Once, Gate, FlipFlop, and MultiGate

**Purpose:** retain local control state. **Pins:** each accepts an exec pulse; Gate also has Open/Close/Toggle, Do Once/MultiGate have Reset. **Configure:** Gate Start Open and MultiGate Loop. **Produces:** the allowed output. **Capabilities:** none. **Failure:** state resets at the run boundary. **Example:** Button Trigger → Do Once → HTTP Request.

## Flow Reroute, Break, and Return

**Purpose:** reroute an exec wire, break the innermost loop, or finish a function/pipeline. **Pins:** exec input and, for Reroute, Then output. **Configure:** none. **Produces:** control flow only. **Capabilities:** none. **Failure:** Break outside a loop is an execution error. **Example:** Branch False → Break inside For Each.

## Set Variable

**Purpose:** store a value for this active execution. **Pins:** Exec/Value inputs; Then exec and Value data outputs. **Configure:** variable name. **Produces:** stored value. **Capabilities:** none. **Failure:** it must be reached before Get Variable can use its result. **Example:** HTTP Result → Set Variable `Response` → later Get Variable.

## Set Global Variable

**Purpose:** write a workspace variable shared by every pipeline and run, surviving an application restart. **Pins:** Exec/Value inputs; Then exec and Value data outputs. **Configure:** pick a declared variable name and an operation — Set overwrites, Increment atomically adds to a number, Append atomically extends a list. **Produces:** the value after the operation, ready for the next step. **Capabilities:** none. **Failure:** a type mismatch or unknown variable stops the run; only the declared type, number for Increment, and list for Append are accepted. **Example:** Cron Trigger → Set Global Variable `lastRun` (operation: Set).
