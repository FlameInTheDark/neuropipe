# Data nodes

Data nodes are pure unless noted. They have no exec pins and calculate only when a consumer asks for an output. Their values are memoized inside the active run frame only.

## Constant

**Purpose:** supply a literal. **Pins:** Value output (Any). **Configure:** value. **Produces:** the configured value. **Capabilities:** none. **Failure:** cast later if a consumer needs a narrower type. **Example:** Constant `5` → For Loop Last Index.

## Format Text

**Purpose:** combine a format string and value. **Pins:** Value input, Text output. **Configure:** format. **Produces:** text. **Capabilities:** none. **Failure:** use Cast for explicit non-text conversion. **Example:** Get Field output → Format Text → Report Markdown.

## Get Field

**Purpose:** expose one or more paths from an object/list. **Pins:** Source input, configured typed outputs. **Configure:** stable output ID, label, path, and data type for each row. **Produces:** each matching value. **Capabilities:** none. **Failure:** a missing path returns null, so inspect source fields and spelling. **Example:** Terminal Result → Get Field `terminal.output` → LLM Prompt.

## Build Object

**Purpose:** make an object from configurable typed input pins. **Pins:** one input per configured field and an Object output. **Configure:** stable input ID, display name, object key/path, and data type. **Produces:** an object; dotted keys create nested objects. **Capabilities:** none. **Failure:** empty, duplicate, or overlapping keys are invalid. **Example:** `Name` → `customer.name`, `Email` → `customer.email` → Build Object.

## Break Object

**Purpose:** split a structured object into named typed outputs. **Pins:** Source object input and configured outputs. **Configure:** stable output ID, display name, dotted key path, and data type. Use **Auto-configure** after connecting a first-party result with known fields. **Produces:** each requested value. **Capabilities:** none. **Failure:** a missing path is null; a value that does not match its configured type stops the requesting path. **Example:** Terminal Result → Break Object (auto-configure) → `terminal.output` → LLM Prompt.

## Cast

**Purpose:** make a type conversion visible. **Pins:** Value input/output. **Configure:** target text, number, or Boolean. **Produces:** converted value. **Capabilities:** none. **Failure:** invalid numeric/Boolean text reports an evaluation error. **Example:** Get Field text → Cast number → Greater Than.

## Query JSON

**Purpose:** read one dotted JSON path. **Pins:** Source input, Value output. **Configure:** optional path. **Produces:** Any. **Capabilities:** none. **Failure:** unknown paths return null. **Example:** HTTP Result → Query JSON `json.items` → For Each Loop.

## Equals

**Purpose:** compare two values. **Pins:** Left/Right inputs, Equal Boolean output. **Configure:** none. **Produces:** Boolean. **Capabilities:** none. **Failure:** compare compatible shapes deliberately. **Example:** Get Field status + Constant `ready` → Branch Condition.

## Greater Than

**Purpose:** compare numbers. **Pins:** Left/Right number inputs, True Boolean output. **Configure:** none. **Produces:** Boolean. **Capabilities:** none. **Failure:** incompatible data types are validation errors. **Example:** Get Field count → Greater Than → Branch.

## Parse JSON

**Purpose:** parse JSON text. **Pins:** Text input, Value output. **Configure:** none. **Produces:** object or list. **Capabilities:** none. **Failure:** malformed JSON stops the requesting path. **Example:** Read File content → Parse JSON → Query JSON.

## Get Variable

**Purpose:** read a value stored earlier in this run. **Pins:** Value output. **Configure:** variable name. **Produces:** Any. **Capabilities:** none. **Failure:** an unknown name returns no value; execute Set Variable first. **Example:** Get Variable `CustomerName` → Format Text.

## Get Global Variable

**Purpose:** read a workspace variable shared by every pipeline and run. **Pins:** Value output. **Configure:** pick a declared variable name. **Produces:** the declared data type. **Capabilities:** none. **Failure:** an unknown or deleted variable stops the run; a read before any write returns the declared default. **Example:** Get Global Variable `visits` → Math Add → Set Global Variable.

## Data Reroute

**Purpose:** keep data wires legible. **Pins:** Value input/output. **Configure:** none. **Produces:** the same value. **Capabilities:** none. **Failure:** none beyond source evaluation. **Example:** LLM result → Data Reroute → Create Report.

## Bytes To Base64

**Purpose:** encode connected raw bytes as a Base64 string. **Pins:** Bytes input, Text output. **Configure:** none. **Produces:** standard Base64 text. **Capabilities:** none. **Failure:** non-bytes input is rejected. **Example:** Read File `bytes` → Bytes To Base64 → Set Field on JSON body.

## Base64 To Bytes

**Purpose:** decode a Base64 string to raw bytes, auto-detecting standard and URL-safe variants. **Pins:** Text input, Bytes output. **Configure:** none. **Produces:** the decoded bytes. **Capabilities:** none. **Failure:** malformed Base64 stops the requesting path. **Example:** HTTP body → Base64 To Bytes → Write File `bytes` content.
