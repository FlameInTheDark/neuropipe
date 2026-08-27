# KV Publish

## Purpose

Publishes a message on a pub/sub channel with `PUBLISH` and reports how many
clients received it. Together with the KV Subscribe Trigger this builds
event-driven automations across processes: one pipeline announces, any number
of listeners react.

## Parameters and results

The **Channel** and **Message** inputs can be wired for dynamic payloads.
**Receivers** is the delivery count reported by the server; zero means
nobody was subscribed at that moment, which is not an error.
