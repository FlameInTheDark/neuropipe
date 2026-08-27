# KV Server Info

## Purpose

Reads the connected server's identity and health summary with `INFO` and
`DBSIZE`. Use it for health checks at the top of a pipeline, capacity logging
in reports, or adapting behaviour to the server flavour.

## Parameters and results

The **Info** object carries version, flavour, uptime, connected clients,
memory usage, and key count. The same values are also exposed as flat
**Version**, **Flavor**, and **Key count** outputs for direct wiring.
Fields a server flavour does not report are simply absent.
