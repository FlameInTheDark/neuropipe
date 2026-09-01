# Data nodes

Data nodes are pure unless noted. They have no exec pins and calculate only when a consumer asks for an output. Their values are memoized inside the active run frame only. Array-specific nodes (build, append, pick, sort, split, reverse, slice, unique) have their own reference under Arrays.

## Constant

**Purpose:** supply a literal. **Pins:** Value output (Any). **Configure:** value. **Produces:** the configured value. **Capabilities:** none. **Failure:** cast later if a consumer needs a narrower type. **Example:** Constant `5` → For Loop Last Index.

## Format Text

**Purpose:** combine a format string and value. **Pins:** Value input, Text output. **Configure:** format. **Produces:** text. **Capabilities:** none. **Failure:** use Cast for explicit non-text conversion. **Example:** Get Field output → Format Text → Report Markdown.

## Get Field

**Purpose:** expose one or more paths from an object/list. **Pins:** Source input, configured typed outputs. **Configure:** stable output ID, label, path, and data type for each row. **Produces:** each matching value. **Capabilities:** none. **Failure:** a missing path returns null, so inspect source fields and spelling. **Example:** Terminal Result → Get Field `terminal.output` → LLM Prompt.

## Build Object

**Purpose:** make an object from configurable typed input pins. **Pins:** one input per configured field and an Object output. **Configure:** stable input ID, display name, object key/path, and data type. **Produces:** an object; dotted keys create nested objects. **Capabilities:** none. **Failure:** empty, duplicate, or overlapping keys are invalid. **Example:** `Name` → `customer.name`, `Email` → `customer.email` → Build Object.

## Build Map

**Purpose:** assemble a flat string-keyed map whose values share one value type. **Pins:** one input per configured entry, all typed by the value type, and a Map output. **Configure:** the value type (any allows mixed values), plus per-row stable input ID, display name, verbatim key, and optional constant. **Produces:** an object with the keys exactly as written — no dotted nesting (use Build Object for paths). **Capabilities:** none. **Failure:** duplicate keys or an entry with neither wire nor constant stops the requesting path. **Example:** value type text, wired Id + constant `EUR` → Build Map → HTTP body.

## Break Object

**Purpose:** split a structured object into named typed outputs. **Pins:** Source object input and configured outputs. **Configure:** stable output ID, display name, dotted key path, and data type. Use **Auto-configure** after connecting a first-party result with known fields. **Produces:** each requested value. **Capabilities:** none. **Failure:** a missing path is null; a value that does not match its configured type stops the requesting path. **Example:** Terminal Result → Break Object (auto-configure) → `terminal.output` → LLM Prompt.

## Cast

**Purpose:** make a type conversion visible. **Pins:** Value input/output. **Configure:** target text, number, Boolean, object, list, or bytes. **Produces:** converted value. **Capabilities:** none. **Failure:** invalid numeric/Boolean text and mismatched shapes (a number to object, an object to list) report an evaluation error. **Example:** Pick from Array → Cast object → KV Hash Set Fields.

## Query JSON

**Purpose:** read one value from JSON data with a JSONPath expression. **Pins:** Source input, Value output. **Configure:** optional JSON path such as `$.geonames[0].lng` — selectors include indexes, wildcards, slices, unions, recursive descent, and `[?(@.lng > 35)]` filters; plain dotted paths (`geonames.0.lng`) still work. **Produces:** the matched value itself, a list when several values match, null when nothing matches. **Capabilities:** none. **Failure:** unknown or invalid paths return null. **Example:** HTTP Result → Query JSON `$.items[0].name` → For Each Loop.

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

## Bytes To Base64

**Purpose:** encode connected raw bytes as a Base64 string. **Pins:** Bytes input, Text output. **Configure:** none. **Produces:** standard Base64 text. **Capabilities:** none. **Failure:** non-bytes input is rejected. **Example:** Read File `bytes` → Bytes To Base64 → Set Field on JSON body.

## Base64 To Bytes

**Purpose:** decode a Base64 string to raw bytes, auto-detecting standard and URL-safe variants. **Pins:** Text input, Bytes output. **Configure:** none. **Produces:** the decoded bytes. **Capabilities:** none. **Failure:** malformed Base64 stops the requesting path. **Example:** HTTP body → Base64 To Bytes → Write File `bytes` content.
