# Build an AI workflow

Use AI nodes as constrained steps inside visible control flow.

1. Fetch or construct input data.
2. Connect it to a **Structured Extract** or **LLM Prompt** data input.
3. Pulse the AI node through its exec input.
4. For a decision, use **LLM Boolean Router** and wire its True and False exec outputs to explicit actions.

Boolean Router requires the model to call `route({ decision: "true" | "false" })`. Choice Router uses stable option IDs, so changing options can invalidate edges that target those output IDs.

Keep prompts specific, give structured field descriptions where available, and set an appropriate LLM queue limit for local hardware. Review capabilities before trusting scheduled use.
