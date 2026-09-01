# Array nodes

Array nodes are pure: they have no exec pins and calculate only when a consumer asks for an output. Their values are memoized inside the active run frame only. Arrays are homogeneous by construction — Build Array declares one element type for every pin, and the operation nodes pass the elements through unchanged.

## Build Array

**Purpose:** assemble an array whose elements share one element type, like a typed-language array. **Pins:** one input per configured item, all typed by the element type, and an Array output. **Configure:** the element type (any allows mixed types), plus per-row stable input ID, display name, and optional constant for unwired pins. **Produces:** an array in row order. **Capabilities:** none. **Failure:** an item with neither a wire nor a constant stops the requesting path. **Example:** element type text, constant `Weekly digest` + wired Total → Build Array → For Each.

## Append to Array

**Purpose:** append one value or a whole list's elements onto a list. **Pins:** Array input, Value input, Array output. **Configure:** Append mode — Single item nests the value as one element, Array elements concatenates a wired list. **Produces:** the grown list; the input is never mutated. **Capabilities:** none. **Failure:** a non-list Array (or a non-list Value in Array elements mode) stops the requesting path. **Example:** accumulated results → Append to Array (Array elements) → For Each.

## Pick from Array

**Purpose:** read the element at a zero-based index from a list. **Pins:** Array and Index inputs, Value output. **Configure:** index when no Index pin is wired. **Produces:** the picked element (Any). **Capabilities:** none. **Failure:** a negative, fractional, or out-of-range index stops the requesting path. **Example:** Sort Array (descending) → Pick from Array (Index 0) → largest value.

## Sort Array

**Purpose:** return a sorted copy of a list. **Pins:** Array input, Array output. **Configure:** Order — ascending or descending. **Produces:** a stable sort (equal elements keep their order); numbers, text, and Booleans sort within their type. **Capabilities:** none. **Failure:** objects, lists, bytes, or null elements stop the requesting path with the kind named. **Example:** Read Excel Rows totals → Sort Array (descending) → Pick from Array.

## Split Array

**Purpose:** cut a list into consecutive batches of a fixed size. **Pins:** Array and Size inputs, Arrays output. **Configure:** batch size (default ten) when no Size pin is wired. **Produces:** a list of arrays — the final batch may be shorter. **Capabilities:** none. **Failure:** a non-positive or fractional size stops the requesting path. **Example:** Query JSON items → Split Array (Size 25) → For Each → Append Excel Rows.

## Reverse Array

**Purpose:** return a copy of the list with the element order flipped. **Pins:** Array input, Array output. **Configure:** none. **Produces:** the reversed list. **Capabilities:** none. **Failure:** a non-list input stops the requesting path. **Example:** Sort Array (ascending) → Reverse Array.

## Slice Array

**Purpose:** cut a section out of a list by start position and optional length. **Pins:** Array, Start, and Count inputs, Array output. **Configure:** start (default zero) and count (empty means to the end). **Produces:** the section — bounds are clamped, never failed. **Capabilities:** none. **Failure:** negative or fractional settings stop the requesting path. **Example:** Read Excel Rows rows → Slice Array (Start 0, Count 20) → For Each.

## Unique Array

**Purpose:** remove duplicate values, keeping each value's first occurrence. **Pins:** Array input, Array output. **Configure:** none. **Produces:** the deduplicated list in original order; equal JSON content counts as a duplicate. **Capabilities:** none. **Failure:** an element that cannot serialize to JSON stops the requesting path. **Example:** KV Set Members → Unique Array → Sort Array.

Companions outside the category: **Length** (Data) counts list elements, **Text Split** (Text) turns delimited text into a list, **Text Join** (Text) turns a list back into delimited text, and **For Each** (Flow) iterates the elements.
