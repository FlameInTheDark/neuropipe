# Neuropipe overview

Neuropipe is a Windows-first, local-first automation workspace. A pipeline is a Blueprint-style graph: white **exec** wires decide what runs, while coloured **data** wires supply values only when a node needs them.

## The workspace

- **Triggers** exposes published button pipelines and their shortcuts.
- **Pipelines** holds draft and published automations.
- **Functions** contains reusable Blueprint graphs.
- **Reports** keeps local Markdown output from report nodes.
- **Chat** talks to a model or a chat-triggered pipeline.
- **Settings** manages one active provider, local models, permissions, plugins, and the optional API.

## A safe lifecycle

Build and test a draft, then publish a validated revision. Capability-changing publishes require trust again before unattended schedules and webhooks can run. Run data is local and redacted before it is retained.

Continue with [Your first automation](docs:getting-started/first-automation) or read [Blueprint exec and data pins](docs:concepts/blueprint-exec-data).
