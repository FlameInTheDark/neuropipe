# Metrics and privacy

Neuropipe's **Metrics** tab is a local operational dashboard. It helps you
understand whether automations are healthy, where time is spent, and how a
configured model is being used without turning your workspace into a telemetry
product.

## What is collected

Metrics record numerical, time-based facts only:

- Pipeline and node outcome, duration, queue wait, trigger kind, and a safe
  pipeline/node identifier.
- LLM provider/model name, provider-reported input and output token counts,
  queue wait, request duration, outcome, and an optional locally calculated
  price estimate.
- API route category, HTTP status class, duration, and webhook verification
  outcome.
- Report creation, chat activity, and managed-runtime lifecycle counters.
- CPU utilisation and working-set memory for **Neuropipe itself** and its
  managed `llama.cpp` child process only.

## What is never collected

The Metrics store deliberately excludes prompts, model replies, packets,
secrets, headers, URLs, IP addresses, request/response bodies, terminal output,
file contents, and chat message content. Metrics stay in the local SQLite
workspace and are never exported or sent automatically.

## Reading the dashboard

### Run health

**Total runs** includes completed, failed, skipped, and cancelled executions.
**Success rate** is completed runs divided by completed plus failed runs;
skipped and cancelled runs do not count as success or failure. The outcome chart
shows the exact count in each selected time bucket.

### Performance and queues

**Average duration** is the mean terminal execution duration. **P95 duration**
shows the value at or below which 95% of detailed runs completed. P95 is exact
while detailed facts are retained; older history is shown as daily aggregate
averages instead.

**Queue wait** is the time after Neuropipe has accepted a queued run and before
it starts executing. It is separate from the run duration and does not change
the execution record shown elsewhere in the app.

### LLM usage and cost

Token numbers are taken only from usage values returned by the provider:

- OpenAI-compatible providers and managed `llama.cpp` use response `usage`
  fields when present.
- Ollama uses its reported evaluation counters when present.

If a provider does not report usage, Neuropipe shows **not reported** rather
than guessing from text. Local Ollama and managed `llama.cpp` calls are labelled
**Local — no provider billing**. Hosted-model cost is an **estimate**, calculated
only from the optional price cards in **Settings → Metrics**; it is not a billing
record. Calls without a matching card remain unpriced.

### Pipeline health and reliability

The heatmap summarizes the completion rate for each pipeline and time bucket.
Select a pipeline name below it to apply that dashboard filter. The **Needs
attention** table groups failures by a safe pipeline or node label; it never
stores error bodies.

### Runtime health

The runtime chart samples only the desktop process and a `llama.cpp` process
started by Neuropipe. It does not inspect other programs, GPU telemetry, or
machine-wide performance.

## Retention and clearing

By default, detailed numerical facts stay for **30 days**. Older facts are
compacted to daily aggregates, which stay for **12 months**. You can adjust both
durations and the owned-process sampling interval in **Settings → Metrics**.
Use **Clear metrics** there to permanently remove metric facts and rollups; it
does not delete pipelines, runs, reports, or chat history.
