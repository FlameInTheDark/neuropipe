# Plugin system

Neuropipe plugins are local, independently versioned bundles. A bundle has a
manifest named plugin.json, an executable sidecar, optional node declarations,
and optional Markdown documentation. Users select the plugin root in
**Settings → Extensions** and explicitly ask Neuropipe to rediscover bundles.
The desktop renderer never reads a plugin directory directly.

## Current v1 boundary

Neuropipe currently provides **bundle discovery, diagnostics, and documentation
loading**:

- It recursively scans the configured plugin root for plugin.json files.
- It checks the manifest identity, API version, and declared sidecar file.
- It exposes the name, version, description, declared node count, and health
  state in Settings.
- It safely loads optional Markdown from enabled, healthy bundles into
  Documentation.

The current host does **not yet** launch a plugin sidecar, register manifest
nodes in the Library, or execute a plugin action. A green **Healthy** status
means that discovery succeeded; it is not a sidecar startup or action self-test.

This distinction matters when developing plugins. You can create a working
documentation/discovery bundle today. Do not expect a declared action, trigger,
or tool node to run until the Emerald-style runtime described below is
implemented.

## Bundle layout

Place each bundle below the configured plugin root. Nested bundles are allowed;
directories beginning with a dot are skipped during discovery.

~~~text
<plugin-root>/
  acme-status/
    plugin.json
    sidecar.exe
    docs/
      status.md
~~~

The default root is Neuropipe's application-data plugins folder. A relative
executable path resolves from the folder containing plugin.json, so a complete
bundle can be copied to another computer. An absolute path is accepted by the
current validator but is not portable and should be avoided in distributed
bundles.

## Manifest reference

The manifest is JSON. These fields are required by current discovery:

| Field | Requirement | Meaning |
| --- | --- | --- |
| id | Required | Stable plugin identifier. Keep it unique and unchanged after release. |
| name | Required | Human-readable name shown in Settings. |
| apiVersion | Required | Must be exactly v1. |
| executable | Required | Existing, non-directory sidecar file. Relative paths resolve from the manifest directory. |

Version and description are strongly recommended for useful diagnostics. The
other fields are optional declarations:

~~~json
{
  "id": "acme-status",
  "name": "Acme Status",
  "version": "0.1.0",
  "description": "Local status checks for Acme services.",
  "apiVersion": "v1",
  "executable": "sidecar.exe",
  "args": ["--serve"],
  "nodes": [],
  "documentation": []
}
~~~

The manager reads args but does not start the executable yet. Do not put
secrets, access tokens, private URLs, or customer data in the manifest: it is
ordinary local JSON and may appear in diagnostics.

## Current node declaration contract

The public package pkg/pluginapi defines Bundle and NodeSpec. A NodeSpec has
an id, kind, label, description, icon, color, capabilities, output metadata,
and configuration-field metadata.

~~~json
{
  "id": "status-check",
  "kind": "action",
  "label": "Status Check",
  "description": "Check a service status endpoint.",
  "icon": "heart-pulse",
  "color": "#60a5fa",
  "capabilities": ["network"],
  "outputs": [{"id": "result", "label": "Result", "kind": "data"}],
  "fields": [{"name": "url", "label": "URL", "kind": "string", "required": true, "secret": false}]
}
~~~

At present, these declarations only contribute to the displayed node count.
They do not create an editor node or validate an action configuration. Keep ids
stable anyway: they are the correct forward-compatible identity boundary.

The package also includes this Go Action interface:

~~~go
type Action interface {
    Validate(ctx context.Context, config json.RawMessage) error
    Execute(ctx context.Context, config json.RawMessage, input map[string]any) (map[string]any, error)
}
~~~

It is a local Go contract, not an interprocess protocol. A separate executable
cannot implement it merely by declaring the type; the current desktop host has
no sidecar launch, handshake, RPC client, or action-execution path.

## Emerald-compatible runtime direction

Emerald uses a Go-first sidecar architecture built on HashiCorp go-plugin with
gRPC transport. Neuropipe should use the same model rather than Go's native
plugin package:

1. Discover plugin.json and start one **managed, long-lived** sidecar per
   trusted bundle.
2. Complete a fixed go-plugin handshake and allow only gRPC transport.
3. Call **Describe** and verify the runtime plugin id and API version against
   the manifest before registering any nodes.
4. Register verified node definitions in the shared Blueprint library and
   context-menu search.
5. Call **ValidateConfig** before publication or execution and invoke actions
   through a cancellable **ExecuteAction** RPC.
6. Support agent tools with **ToolDefinition** and **ExecuteTool** RPCs.
7. Keep one bidirectional **TriggerRuntime** stream per trigger-capable plugin,
   delivering full subscription snapshots and receiving emitted trigger events.
8. Stop sidecars on reload, disable, failure, and desktop shutdown; never
   leave a plugin-owned process unmanaged.

This is not available in the current build, so no public Neuropipe gRPC
schema or Go SDK has been released yet. The existing manifest and action types
are intentionally smaller than Emerald's runtime contract. Treat this section
as the compatibility direction, not as a callable API.

When implemented, the runtime should adopt Blueprint-specific constraints:

- Plugin actions must expose typed exec and data pins, not packet-only ports.
- Plugin data reads must respect one-run cache scopes and never trigger an
  impure action on demand.
- Plugins must declare minimum capabilities; manual runs receive the same
  approval preview and published revision trust rules as core nodes.
- Trigger subscription snapshots must refer to exact published revisions.
- Cancellation, execution budgets, error redaction, metrics, and queue limits
  must cross the RPC boundary.
- A failed or incompatible plugin must make dependent graph nodes unavailable
  rather than silently substitute behavior.

## Plugin documentation

Bundles can contribute local Markdown to the Documentation tab. Each entry has
a stable id, title, category path, optional summary, relative Markdown path,
and optional associated node types:

~~~json
{
  "documentation": [
    {
      "id": "status-check",
      "title": "Status Check",
      "categoryPath": ["Extensions", "Acme Status"],
      "summary": "Configure and interpret the Acme Status Check.",
      "path": "docs/status.md",
      "nodeTypes": ["acme-status:status-check"]
    }
  ]
}
~~~

Neuropipe exposes it as plugin:<plugin-id>:<document-id> and only lists pages
from healthy bundles. Documentation paths must meet all of these checks:

- A path is relative and ends in .md.
- Its resolved location stays inside the bundle.
- Its document id is unique within the bundle.
- The file exists, is not a directory, and is at most 1 MiB.

A documentation failure does not disable an otherwise valid plugin. Settings
shows a documentation diagnostic while the plugin remains discoverable.
Markdown is rendered through Neuropipe's shared safe renderer; it never
executes bundle code or lets the renderer read arbitrary local files.

## Health, security, and updates

Click **Rediscover plugins** after adding, replacing, or removing a bundle.
Rediscovery is not a trust grant and does not start a sidecar in the current
build. Plugin files are local, user-installed code; review their source and
provenance before installing them.

| Status | Meaning | Next step |
| --- | --- | --- |
| Healthy | id, name, API version v1, and an available sidecar file passed discovery. | Read docs or continue packaging the bundle. |
| Invalid manifest | JSON failed to parse, required identity is missing, or API version is unsupported. | Correct plugin.json and rediscover. |
| Sidecar unavailable | The declared file is absent or is a directory. | Package the executable with the bundle and check its path. |
| Documentation diagnostic | Bundle discovery passed but a docs entry was rejected. | Correct metadata, relative path, or file size and rediscover. |

Keep plugin ids, node ids, output ids, and document ids stable. Use semantic
versions, test a copied bundle in a clean plugin root, and never assume that
discovery acceptance is equivalent to executable compatibility.

## Related reading

- [Build your first plugin](docs:extensions/first-plugin)
- [Publishing and trust](docs:concepts/publishing-trust)
