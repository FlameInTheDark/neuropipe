# Runs and queues

Each trigger creates a local execution record with node-level inputs, outputs, timings, and redacted errors. The bottom execution log highlights the active exec path and can inspect stored values without writing them onto the canvas.

## Queues and cancellation

Neuropipe owns bounded execution and LLM queues. Per-pipeline concurrency normally skips a run while the same pipeline is active; that skipped run is still recorded. The LLM limit is configured independently so a single local llama.cpp server can process model nodes one at a time.

Cancellation propagates through the run context to supported HTTP calls, managed child processes, and queued model work. Loop nodes enforce bounds and check cancellation between iterations.

Execution history is redacted and cleaned according to the retention setting. It is never a cache for a future run.
