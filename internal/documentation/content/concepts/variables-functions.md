# Variables and functions

## Execution variables

**Set Variable** stores a typed value under a name for the current run. **Get Variable** is pure and retrieves it later. Values do not cross pipeline runs, conversations, schedules, or loop frames unless they are explicitly carried as data.

Use variables for a value that is needed far from its original producer. For direct dependencies, prefer a visible data wire.

## Custom functions

Functions are reusable graphs with a versioned signature. An impure function starts at **Function Entry** and ends at **Function Return**. A pure function uses **Function Inputs** and **Function Outputs**, evaluates only when a caller requests an output, and shares the caller’s execution variables.

Function pin edits synchronize call nodes. Publishing a signature-breaking change flags callers until their wires are repaired. Recursion, including indirect recursion, is rejected. A referenced function cannot be deleted.
