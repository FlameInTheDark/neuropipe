# Build a common pipeline

This pattern fetches an API, extracts a value, and writes a short report.

1. Start with a **Button Trigger**.
2. Pulse **HTTP Request** from its Start pin.
3. Connect HTTP Request’s Result to **Get Field** Source and configure a `json.summary` output.
4. Connect that output to **Format Text**, then to Create Report’s Markdown input.
5. Connect HTTP Request’s exec output to **Create Report**.

The data nodes evaluate only when Create Report needs Markdown. The HTTP action must already have executed, because data wiring cannot invoke it.

If you later move the report creation to another exec branch, wire the required action on that branch too or store a deliberate value with **Set Variable**.
