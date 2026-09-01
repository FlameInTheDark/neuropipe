# Providers and models

Neuropipe supports several configured LLM providers at once. Manage them in **Settings → Provider**: add providers, pick the default one used when an AI node makes no explicit choice, and configure each provider's available models.

## Supported provider kinds

- **Ollama** connects to an already-running local Ollama endpoint.
- **Managed llama.cpp** downloads a Neuropipe-owned runtime and GGUF model, then binds it to loopback only.
- **OpenAI-compatible** connects to a user-configured compatible endpoint (OpenRouter, Groq, LM Studio, and friends).
- **Anthropic** connects to the Anthropic Messages API with its own key handling.

## Configuring a provider

Each provider keeps its own base URL, API key (stored as a secret), default model, and model list. Pick a saved secret as the API key; the key itself never leaves the local protected vault and is sent only to that provider.

The **model list** powers the Model picker of AI nodes. Press **Discover models** to load the provider's advertised models when its API supports listing (Ollama, OpenAI-compatible, and Anthropic do). When discovery is unavailable — or you want a model the API does not advertise — add entries manually with a **model key** (the exact model ID the provider expects) and an optional display **title**. The provider's **default model** answers any AI node that does not select one explicitly.

The model list and the provider card collapse behind their headers, so a long configuration stays compact; a count badge shows how many models or overrides are configured inside.

## Generation parameters

Every provider carries an optional set of **generation parameters** under its card, and every model entry carries its own overrides behind the sliders button. Values that are left empty are simply not sent, so the provider keeps its own defaults; a value set on a model wins over the provider value for that field.

- **Temperature** (0–2) scales sampling randomness.
- **Top P** (0–1) enables nucleus sampling; **Top K** limits sampling to the K most likely tokens.
- **Max tokens** caps the completion length — useful for local models whose provider reports no limit (OpenAI `max_tokens`, Anthropic `max_tokens`, Ollama `num_predict`).
- **Context size** widens the prompt window when it was not discovered from the provider (Ollama `num_ctx`); for the managed llama.cpp provider it is the window llama-server is launched with for that model.

## Managed llama.cpp as a provider

**Add provider → Managed llama.cpp** puts the app-owned runtime into the provider list like any other provider, so AI nodes can pin it explicitly and it can be the default. It appears automatically as well: installing or selecting a model in **Settings → Models** adds it and makes it the default.

The managed provider has no base URL or API key to edit — Neuropipe owns the endpoint. Its model list is not edited by hand either: every model downloaded in **Settings → Models** is listed automatically, and the list stays in sync as models are installed or removed. Pick any of them as the provider's default model.

The runtime starts on demand: the first request routed to the managed provider starts llama-server (or switches it to the requested model) and reuses the running server afterwards. Starting the runtime explicitly in **Settings → Runtime** remains the way to pin a session's model and to watch its status, and routing never changes the default provider — that choice stays yours. Removing the managed provider from the list is honored — it stays hidden until added back — and a model's **context size** override is the window llama-server is launched with.

**Settings → Runtime** keeps a live release list even when GitHub's REST API is rate-limited or blocked: the catalog first tries the API, then — exactly like a browser — reads the repository's public releases feed and asset pages on github.com, and finally falls back to the last successful list cached on disk. Installed runtimes are listed with their acceleration builds either way, and the page's banner explains which source served the list while the API is down. Outbound catalog traffic (release lookups and runtime/model downloads) also honors the Windows system proxy, so a desktop behind Clash, v2rayN, or a corporate forward proxy reaches the same hosts the browser reaches; use the page's refresh button to force a fresh live lookup.

The installed-runtime picker shows exactly the runtime the app will launch: installed builds lead the list with their acceleration modes, switching to another installed build keeps the pinned mode, and choosing a release that is not installed yet selects it with automatic mode resolution. When no runtime is configured the field stays empty instead of implying the newest release.

## Using providers in AI nodes

Every AI node has **Provider** and **Model** pickers in the inspector. Both start empty, which follows the app's default provider and that provider's default model, so re-pointing the default re-routes all unopinionated nodes. Select a provider on the node to pin it, then pick one of its configured models. A wired **Model** input pin still overrides the selection at runtime.

The Model picker is searchable: type a fragment of the display title or of the model key to filter long model lists, which matters for providers that advertise dozens of models. The match counter shows how many entries survived the filter, and the arrow, Home, and End keys walk the filtered list. The same search is available on the default-model picker in **Settings → Provider**.

## Local model setup

In **Settings → Models**, search public GGUF repositories, select a quantisation, and install it into the configured content folder. Installation verifies LFS SHA-256 when the Hub provides it and stores local metadata beside the model. Pick the installed model in **Settings → Runtime**, then start the managed runtime.

The LLM queue limits concurrent model work. Set it to one for a local runtime that cannot serve parallel requests.

Provider credentials stay in the Windows-protected vault. Node previews, logs, exports, and plugin diagnostics redact resolved secrets.
