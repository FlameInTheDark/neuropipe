/**
 * Bytes-pin live entry — renders the REAL NodeCard component (pin dots +
 * hover tooltips) for a mini graph whose nodes carry bytes pins, mirroring
 * the current Go definitions: storage Upload File (Data input, bytes),
 * Draw Image (Image output, bytes), Base64 To Bytes (Result output, bytes)
 * and a text pin for contrast. The harness script hovers each port dot and
 * screenshots the tooltip, which must read "Bytes" (i18n pins.type_bytes).
 */
import { createRoot } from "react-dom/client";
import { useMemo } from "react";
import "../src/i18n";
import { NodeCard } from "../src/components/NodeCard";
import { portFromNodePort, type DefinitionIndex } from "../src/lib/adapters";
import { pinPalette, ASSIGNABLE_PIN_TYPES } from "../src/lib/pins";
import type { NodeDefinition, NodePort } from "../src/lib/types";
import type { GraphNode } from "../src/types";

/* ------------------------------------------------------------------ */
/* definitions mirrored from the Go packages (post-bytes-label)        */
/* ------------------------------------------------------------------ */

const exec = (id: string, label: string, direction: NodePort["direction"]): NodePort => ({
  id, label, kind: "exec", direction, color: "#fafafa", maxConnections: 1,
});
const bytesPin = (id: string, label: string, direction: NodePort["direction"]): NodePort => ({
  id, label, kind: "data", direction, dataType: "bytes", type: { kind: "bytes" }, color: "#fbbf24", maxConnections: 1,
});
const textPin = (id: string, label: string, direction: NodePort["direction"]): NodePort => ({
  id, label, kind: "data", direction, dataType: "text", type: { kind: "string" }, color: "#e879f9", maxConnections: 1,
});

const uploadDefinition: NodeDefinition = {
  type: "action:storage_upload_file",
  label: "Upload File",
  category: "Storage",
  icon: "cloud",
  color: "#f59e0b",
  mode: "impure",
  inputs: [
    exec("in", "Exec", "input"),
    textPin("localPath", "Local path", "input"),
    bytesPin("data", "Data", "input"),
    textPin("base64", "Base64", "input"),
    textPin("remotePath", "Remote path", "input"),
  ],
  outputs: [exec("out", "Then", "output")],
  fields: [],
  defaultConfig: { source: "", storageId: "stg-1", remotePath: "out.bin" },
};

const drawImageDefinition: NodeDefinition = {
  type: "action:draw_image",
  label: "Draw Image",
  category: "Image",
  icon: "image",
  color: "#38bdf8",
  mode: "impure",
  inputs: [exec("in", "Exec", "input"), textPin("outputPath", "Output path", "input")],
  outputs: [exec("out", "Then", "output"), bytesPin("image", "Image", "output"), textPin("base64", "Base64", "output")],
  fields: [],
  defaultConfig: {},
};

const base64ToBytesDefinition: NodeDefinition = {
  type: "data:base64_to_bytes",
  label: "Base64 To Bytes",
  category: "Data",
  icon: "binary",
  color: "#22c55e",
  mode: "pure",
  inputs: [textPin("value", "Value", "input")],
  outputs: [bytesPin("result", "Result", "output")],
  fields: [],
  defaultConfig: {},
};

const definitions: DefinitionIndex = {
  [uploadDefinition.type]: uploadDefinition,
  [drawImageDefinition.type]: drawImageDefinition,
  [base64ToBytesDefinition.type]: base64ToBytesDefinition,
};

function nodeFor(id: string, definition: NodeDefinition, x: number, y: number): GraphNode {
  return {
    id,
    type: definition.type,
    title: definition.label,
    icon: definition.icon,
    group: definition.category,
    summary: definition.description,
    x,
    y,
    status: "idle",
    inputs: definition.inputs.map(portFromNodePort),
    outputs: definition.outputs.map(portFromNodePort),
    fields: [],
    values: { ...definition.defaultConfig },
    outputSchema: [],
  };
}

/* ------------------------------------------------------------------ */
/* probe: palette + computed colours                                   */
/* ------------------------------------------------------------------ */

function PaletteProbe() {
  const rows = useMemo(() => {
    const probe = document.createElement("span");
    probe.className = "h-2 w-2 rounded-full";
    probe.style.background = "var(--pin-bytes)";
    document.body.appendChild(probe);
    const computed = getComputedStyle(probe).backgroundColor;
    probe.remove();
    return [
      { key: "bytes base var", value: computed },
      { key: "bytes palette dot", value: pinPalette("bytes").dot },
      { key: "bytes palette name", value: pinPalette("bytes").name },
      { key: "bytes in ASSIGNABLE_PIN_TYPES", value: String(ASSIGNABLE_PIN_TYPES.includes("bytes")) },
    ];
  }, []);
  return (
    <div className="absolute top-2 left-[700px] w-[280px] rounded-lg border border-ink-700 bg-ink-850/95 p-3 text-[11px] text-fg-subtle">
      <p className="mb-2 font-medium text-fg">bytes palette probe</p>
      {rows.map((r) => (
        <p key={r.key} className="font-mono text-[10px] leading-relaxed">
          <span className="text-fg-faint">{r.key}: </span>
          <span className="text-fg">{r.value}</span>
        </p>
      ))}
      <div className="mt-2 flex items-center gap-1.5">
        <span className="h-2 w-2 rounded-full" style={{ background: "var(--pin-bytes)" }} />
        <span className="h-2 w-2 rounded-full" style={{ background: "var(--pin-bytes-strong)" }} />
        <span className="h-2 w-2 rounded-full" style={{ background: "var(--pin-bytes-deep)" }} />
        <span className="ml-1 font-mono text-[10px] text-fg-faint">--pin-bytes / strong / deep</span>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* page                                                                */
/* ------------------------------------------------------------------ */

function App() {
  const nodes: GraphNode[] = [
    nodeFor("upload", uploadDefinition, 24, 120),
    nodeFor("draw", drawImageDefinition, 24, 420),
    nodeFor("b64", base64ToBytesDefinition, 320, 640),
  ];
  const connected = new Set<string>(["upload:data:in", "draw:image:out", "b64:result:out"]);
  return (
    <div className="relative h-screen w-screen overflow-hidden bg-ink-900">
      <div className="absolute top-2 left-4 z-20 text-[12px] text-fg-subtle">
        Bytes pin live check — hover the <span className="font-mono text-fg">Data</span>,{" "}
        <span className="font-mono text-fg">Image</span> and{" "}
        <span className="font-mono text-fg">Result</span> dots
      </div>
      <PaletteProbe />
      {nodes.map((n) => (
        <NodeCard
          key={n.id}
          node={n}
          selected={false}
          connectedPorts={connected}
          onPointerDown={() => {}}
          onSelect={() => {}}
        />
      ))}
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
