# Variables and functions

## Execution variables

**Set Variable** stores a typed value under a name for the current run. **Get Variable** is pure and retrieves it later. Values do not cross pipeline runs, conversations, schedules, or loop frames unless they are explicitly carried as data.

Use variables for a value that is needed far from its original producer. For direct dependencies, prefer a visible data wire.

## Global variables

**Global variables** are workspace-scoped and survive an application restart. They are declared once on the **Variables** screen with an immutable name and data type, then read with **Get Global Variable** and written with **Set Global Variable**. A pipeline that only reads sees the declared default until a run has written a value.

Values live in memory under a single lock, so two pipelines running at once never corrupt each other. Writes are persisted to the local database at most once a second and once more when the app closes; a crash can lose the last sub-second of writes. Set's **Increment** and **Append** operations are atomic: use them instead of a Get → update → Set chain whenever multiple pipelines, schedules, or triggers can work with the same variable concurrently. Delete is guarded while any draft or published definition still references the variable.

## Custom functions

Functions are reusable graphs with a versioned signature. An impure function starts at **Function Entry** and ends at **Function Return**. A pure function uses **Function Inputs** and **Function Outputs**, evaluates only when a caller requests an output, and shares the caller’s execution variables.

Function pin edits synchronize call nodes. Publishing a signature-breaking change flags callers until their wires are repaired. Recursion, including indirect recursion, is rejected. A referenced function cannot be deleted.
