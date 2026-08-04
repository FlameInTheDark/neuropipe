# Providers and models

Neuropipe has one active provider at a time. Choose it in **Settings → Provider**.

## Supported modes

- **Ollama** connects to an already-running local Ollama endpoint.
- **Managed llama.cpp** downloads a Neuropipe-owned runtime and GGUF model, then binds it to loopback only.
- **OpenAI-compatible** connects to a user-configured compatible endpoint.

## Local model setup

In **Settings → Models**, search public GGUF repositories, select a quantisation, and install it into the configured content folder. Installation verifies LFS SHA-256 when the Hub provides it and stores local metadata beside the model. Pick the installed model in **Settings → Runtime**, then start the managed runtime.

The LLM queue limits concurrent model work. Set it to one for a local runtime that cannot serve parallel requests.

Provider credentials stay in the Windows-protected vault. Node previews, logs, exports, and plugin diagnostics redact resolved secrets.
