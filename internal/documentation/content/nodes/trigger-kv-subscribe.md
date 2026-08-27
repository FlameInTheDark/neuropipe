# KV Subscribe Trigger

## Purpose

Starts a published pipeline when a pub/sub message arrives on the selected KV
database. Channels can be listed explicitly or matched with Redis glob
patterns such as `events:*`. Because delivery is unattended, the trigger
follows the publishing trust model: the pipeline revision must be trusted
and the binding enabled before messages start pipelines.

## Parameters and results

Choose the KV database, then set channels or patterns as a comma-separated
list. Every delivered message exposes **Channel**, **Message**, **Pattern**
(empty for exact subscriptions), and **Received at** data outputs. Bindings
that share a database and channel set share one subscription connection.
