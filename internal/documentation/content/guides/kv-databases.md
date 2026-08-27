# Key/value databases

Neuropipe speaks the Redis protocol natively: register a Redis, Valkey, KeyDB,
or Dragonfly server in Datastores and it becomes a first-class citizen —
browseable in the dedicated Keys, Console, and Info tabs, and usable from the
KV node family in any pipeline. When you would rather not run a server at
all, the integrated **SugarDB** engine embeds a full key/value store inside
Neuropipe itself.

## Register a connection

Open **Datastores**, click **New connection**, and pick **Redis / Valkey /
KeyDB**. Enter host and port (6379 by default), an optional username and
password, the logical database index, and whether TLS is required. Advanced
setups can paste a complete `redis://` URL instead. Use **Test connection**
before saving; the password is stored in the local vault and never leaves the
machine.

## Use the embedded SugarDB store

Pick **SugarDB** in the same dialog for an integrated store that needs no
external server. The engine starts inside Neuropipe the first time a node or
the browser touches it, listens only on loopback, and persists every write to
disk through an append-only file plus periodic snapshots, so your data
survives restarts. Leave the data directory empty to keep files under the
app data folder, or point it at any folder you manage yourself; unregistering
the connection never deletes the files. The logical database index works the
same as on Redis servers, and the optional password protects the loopback
listener. Every KV node, the Keys and Console tabs, and the pub/sub trigger
work against SugarDB exactly as they do against remote servers; the browser's
key listing falls back from `SCAN` to `KEYS` automatically because the engine
does not implement `SCAN`, and per-key memory usage is hidden for the same
reason.

## Browse and operate

The **Keys** tab pages through the keyspace with a glob-pattern search and a
type filter, showing each key's type, TTL, and size. Selecting a key opens a
per-type value viewer — strings, hashes, lists, sets, sorted sets, and streams
each get their own rendering — with actions to copy, delete, or change the
TTL. The **Console** tab executes arbitrary commands with the same safety
rules as the KV Command node, and **Info** summarises the server.

## Build with KV nodes

The **KV Store** category covers the common structures with typed pins:
strings (`KV Get`, `KV Set`, counters, TTL), hashes, lists, sets, and sorted
sets, plus `KV Scan` for safe keyspace iteration and `KV Publish` for
announcing events. When a command has no curated node, the generic
**KV Command** node runs it with typed argument pins you declare yourself —
the same pattern as SQL parameters. All of these pick any registered KV
connection, remote or embedded, from the **KV database** field.

## React to pub/sub

Add a **KV Subscribe Trigger** to a pipeline, publish it, then trust the
revision and enable the binding from the browser's Info tab. Messages arrive
as `channel`, `message`, `pattern`, and `receivedAt` outputs, and pipelines
run unattended exactly like scheduled and webhook triggers. The trigger works
against the embedded SugarDB store too, which makes it a zero-infrastructure
event bus for local automations.
