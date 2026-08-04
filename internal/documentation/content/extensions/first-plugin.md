# Build a two-node plugin bundle

This guide builds a realistic **two-node** Weather Tools bundle. It teaches the
v1 bundle format, diagnostics, and Markdown documentation that work today. It
also shows how the two future action handlers can be structured in Go.

> **Important:** the current v1 host discovers bundles and shows their
> documentation, but does **not** launch a sidecar, add plugin nodes to the
> Library, or execute their Go code. The executable in this guide is required
> for discovery only. Treat the handler code as a prepared, tested design for
> the managed gRPC runtime—not a currently runnable extension.

## What you need

- Neuropipe with **Settings → Extensions** available.
- Go 1.26 or a compatible toolchain.
- A writable plugin root. Use the exact folder displayed in Settings.

## 1. Create the bundle layout

Create one self-contained folder under the configured plugin root:

~~~text
weather-tools/
  plugin.json
  bin/
    weather-tools.exe
  cmd/weather-tools/
    main.go
  docs/
    convert-temperature.md
    classify-temperature.md
~~~

All relative paths in `plugin.json` are resolved from the folder containing
that manifest. Keeping the executable and documentation in this tree makes the
bundle safe to copy to another machine.

## 2. Build the discovery sidecar

For the current host, a harmless executable is sufficient. Create
`cmd/weather-tools/main.go`:

~~~go
package main

func main() {
    select {}
}
~~~

From `weather-tools`, build it:

~~~powershell
go build -o .\bin\weather-tools.exe .\cmd\weather-tools
~~~

The empty `select` keeps the process alive **if** a future managed host starts
it. Neuropipe v1 does not start it today, so it cannot consume CPU, access the
network, or perform any other work. Never replace this with an unmanaged
background worker.

## 3. Declare both nodes and their docs

Create `plugin.json` beside `bin` and `docs`:

~~~json
{
  "id": "weather-tools",
  "name": "Weather Tools",
  "version": "0.1.0",
  "description": "Temperature conversion and classification examples.",
  "apiVersion": "v1",
  "executable": "bin/weather-tools.exe",
  "nodes": [
    {
      "id": "convert-temperature",
      "kind": "action",
      "label": "Convert Temperature",
      "description": "Converts a Celsius input to Fahrenheit.",
      "icon": "thermometer",
      "color": "#38bdf8",
      "capabilities": [],
      "outputs": [
        { "id": "fahrenheit", "label": "Fahrenheit", "kind": "data" }
      ],
      "fields": []
    },
    {
      "id": "classify-temperature",
      "kind": "action",
      "label": "Classify Temperature",
      "description": "Labels a Fahrenheit value as cold, mild, or warm.",
      "icon": "cloud-sun",
      "color": "#f59e0b",
      "capabilities": [],
      "outputs": [
        { "id": "band", "label": "Band", "kind": "data" }
      ],
      "fields": [
        { "name": "coldAtOrBelow", "label": "Cold at or below", "kind": "number", "required": false, "secret": false },
        { "name": "warmAtOrAbove", "label": "Warm at or above", "kind": "number", "required": false, "secret": false }
      ]
    }
  ],
  "documentation": [
    {
      "id": "convert-temperature",
      "title": "Convert Temperature",
      "categoryPath": ["Extensions", "Weather Tools"],
      "summary": "Convert a Celsius value to Fahrenheit.",
      "path": "docs/convert-temperature.md",
      "nodeTypes": ["weather-tools:convert-temperature"]
    },
    {
      "id": "classify-temperature",
      "title": "Classify Temperature",
      "categoryPath": ["Extensions", "Weather Tools"],
      "summary": "Classify a Fahrenheit value into a readable band.",
      "path": "docs/classify-temperature.md",
      "nodeTypes": ["weather-tools:classify-temperature"]
    }
  ]
}
~~~

### What each declaration means

- `id` is the permanent bundle identity. Keep it lowercase and stable.
- Each node `id` is stable within that bundle. It is what a future graph will
  persist—not its display label.
- `kind`, `label`, `description`, `icon`, and `color` describe the future
  Library presentation. They are metadata in v1.
- `outputs` document the result-map keys the handler returns. Here they are
  `fahrenheit` and `band`.
- `fields` describe future Inspector configuration. The classifier has two
  optional numeric thresholds; the converter needs none.
- `capabilities` must be an empty array for these pure local examples. Do not
  claim network, file, terminal, or secret access unless a later node truly
  needs it.
- `documentation` maps two safe local Markdown files into the Documentation
  tree. Each `nodeTypes` entry uses `<plugin-id>:<node-id>` so it can later
  connect a node inspector to its reference page.

Only `id`, `name`, `apiVersion: "v1"`, and `executable` are currently required
for discovery. Node declarations increase the displayed declared-node count;
they do **not** create a Library node yet.

## 4. Write the reference pages

Create `docs/convert-temperature.md`:

~~~markdown
# Convert Temperature

## Purpose

Converts a numeric `celsius` input to a numeric `fahrenheit` result.

## Example

Manual Trigger → Convert Temperature → Classify Temperature → Create Report
~~~

Then create `docs/classify-temperature.md`:

~~~markdown
# Classify Temperature

## Purpose

Labels a numeric `fahrenheit` input as `cold`, `mild`, or `warm`.

## Configuration

`coldAtOrBelow` defaults to 50 and `warmAtOrAbove` defaults to 77.
~~~

Paths must be relative, end in `.md`, resolve inside the bundle, and be at
most 1 MiB. A rejected documentation entry produces a documentation diagnostic
but does not make a valid bundle unhealthy.

## 5. Prepare the future handlers

When Neuropipe ships its managed gRPC plugin runtime, the executable will need
handlers. The following Go code illustrates the separation to keep: decode and
validate configuration first, then make `Execute` return a small result map
whose keys match the manifest output IDs.

~~~go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "math"

    "github.com/FlameInTheDark/neuropipe/pkg/pluginapi"
)

type ClassifierConfig struct {
    ColdAtOrBelow float64 `json:"coldAtOrBelow"`
    WarmAtOrAbove float64 `json:"warmAtOrAbove"`
}

type ConvertTemperature struct{}

func (ConvertTemperature) Validate(context.Context, json.RawMessage) error {
    return nil // This node has no configured fields.
}

func (ConvertTemperature) Execute(_ context.Context, _ json.RawMessage, input map[string]any) (map[string]any, error) {
    celsius, err := number(input, "celsius")
    if err != nil { return nil, err }
    return map[string]any{"fahrenheit": celsius*9/5 + 32}, nil
}

type ClassifyTemperature struct{}

func (ClassifyTemperature) Validate(_ context.Context, raw json.RawMessage) error {
    config := ClassifierConfig{ColdAtOrBelow: 50, WarmAtOrAbove: 77}
    if len(raw) != 0 && string(raw) != "null" {
        if err := json.Unmarshal(raw, &config); err != nil { return err }
    }
    if config.ColdAtOrBelow >= config.WarmAtOrAbove {
        return fmt.Errorf("cold threshold must be below warm threshold")
    }
    return nil
}

func (ClassifyTemperature) Execute(_ context.Context, raw json.RawMessage, input map[string]any) (map[string]any, error) {
    fahrenheit, err := number(input, "fahrenheit")
    if err != nil { return nil, err }
    config := ClassifierConfig{ColdAtOrBelow: 50, WarmAtOrAbove: 77}
    if len(raw) != 0 && string(raw) != "null" {
        if err := json.Unmarshal(raw, &config); err != nil { return nil, err }
    }
    band := "mild"
    if fahrenheit <= config.ColdAtOrBelow { band = "cold" }
    if fahrenheit >= config.WarmAtOrAbove { band = "warm" }
    return map[string]any{"band": band}, nil
}

func number(input map[string]any, key string) (float64, error) {
    value, ok := input[key].(float64)
    if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
        return 0, fmt.Errorf("%s must be a finite number", key)
    }
    return value, nil
}

var _ pluginapi.Action = ConvertTemperature{}
var _ pluginapi.Action = ClassifyTemperature{}
~~~

`Validate` protects saved configuration before work starts. The converter reads
the future `celsius` input and produces only `fahrenheit`; the classifier reads
that output, applies validated thresholds, and produces only `band`. `number`
keeps strict numeric handling in one reusable place. The two final assignments
make the compiler verify that both handlers satisfy the documented `Action`
shape.

This is deliberately **not** a sidecar protocol. Do not add a goroutine,
listener, process launcher, or custom RPC loop around it. The host has not
published the handshake or `ExecuteAction` RPC yet.

## 6. Rediscover and inspect

1. Open **Settings → Extensions**.
2. Confirm the plugin root contains `weather-tools`.
3. Select **Rediscover plugins**.
4. Check that **Weather Tools** is **Healthy** and reports **two** declared
   nodes.
5. Open **Documentation → Extensions → Weather Tools** to see both pages.

If the manifest or docs change, rediscover again. A healthy row confirms only
the manifest and sidecar file passed discovery—no code has been launched.

## Troubleshooting

| Symptom | Cause | Fix |
| --- | --- | --- |
| No plugin row | Bundle is outside the configured root. | Select the correct root and rediscover. |
| Invalid manifest | JSON is malformed or required identity is missing. | Check `id`, `name`, `apiVersion`, and JSON syntax. |
| Sidecar unavailable | The `executable` path does not identify a file. | Build `bin/weather-tools.exe` and keep the relative path correct. |
| Healthy, but a docs page is absent | A documentation entry failed validation. | Check unique ids, category path, `.md` path, bundle boundary, and 1 MiB limit. |
| No Library node | Expected v1 behavior. | Plugin node execution has not shipped yet. |

## Before distributing

- Ship the entire `weather-tools` folder, including `bin` and `docs`.
- Preserve plugin, node, output, and documentation IDs across releases.
- Test from a clean plugin root and rediscover after every install.
- Never place a secret, private endpoint, or customer data in `plugin.json` or
  documentation.
- Describe all future capabilities honestly and request the smallest scope.

For the complete current contract and the Emerald-compatible runtime direction,
read the [Plugin system](docs:extensions/plugin-system).
