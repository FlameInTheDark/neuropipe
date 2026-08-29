/**
 * Upload-source live entry — the REAL Inspector component rendered over the
 * REAL adapter pipeline (visibleFields + fieldDefFromConfig + refreshNode)
 * with the merged storage Upload File definition mirrored from the Go
 * package (source dropdown: Auto / From disk / From node / From base64).
 * Switching the Source dropdown must show/hide the matching config fields
 * (inspector) and input pins (left panel) live. The harness script switches
 * dropdown values, then screenshots each state.
 */
import { createRoot } from "react-dom/client";
import { useMemo, useState } from "react";
import "../src/i18n";
import { Inspector } from "../src/components/Inspector";
import { refreshNode, type DefinitionIndex } from "../src/lib/adapters";
import type { ConfigField, NodeDefinition, NodePort } from "../src/lib/types";
import type { GraphNode, LogEntry } from "../src/types";
import type { EditorApi } from "../src/features/graph/PipelineEditor";

/* ------------------------------------------------------------------ */
/* definition mirrored from internal/nodes/storage/transfer.go         */
/* ------------------------------------------------------------------ */

const pin = (id: string, label: string, dataType: NodePort["dataType"] = "text"): NodePort => ({
  id,
  label,
  kind: "data",
  direction: "input",
  dataType,
  color: "#a1a1aa",
  maxConnections: 1,
});

const field = (partial: Partial<ConfigField> & { name: string; label: string }): ConfigField => ({
  kind: "string",
  ...partial,
} as ConfigField);

const uploadDefinition: NodeDefinition = {
  type: "action:storage_upload_file",
  label: "Upload File",
  category: "Storage",
  icon: "cloud",
  color: "#f59e0b",
  mode: "impure",
  inputs: [
    { id: "in", label: "Exec", kind: "exec", direction: "input", color: "#fafafa", maxConnections: 1 },
    pin("localPath", "Local path"),
    pin("data", "Data", "any"),
    pin("base64", "Base64"),
    pin("remotePath", "Remote path"),
    pin("contentType", "Content type"),
  ],
  outputs: [
    { id: "out", label: "Then", kind: "exec", direction: "output", color: "#fafafa", maxConnections: 1 },
    { id: "result", label: "Result", kind: "data", direction: "output", dataType: "object", color: "#60a5fa", maxConnections: 1 },
  ],
  fields: [
    field({ name: "storageId", label: "Storage", kind: "storage-select", required: true }),
    field({ name: "source", label: "Source", kind: "select", options: [
      { value: "", label: "Auto — use whatever is set" },
      { value: "file", label: "From disk" },
      { value: "bytes", label: "From node" },
      { value: "base64", label: "From base64" },
    ] }),
    field({ name: "localPath", label: "Local path", placeholder: "/home/user/pictures/chart.png", visibleWhen: "source=file|source=" }),
    field({ name: "base64", label: "Base64", kind: "textarea", placeholder: "aGVsbG8gd29ybGQ= or a data: URL", visibleWhen: "source=base64|source=" }),
    field({ name: "remotePath", label: "Remote path", placeholder: "reports/2026/chart.png (trailing / keeps the file name)", required: true }),
    field({ name: "contentType", label: "Content type", placeholder: "image/png" }),
  ],
  defaultConfig: {
    storageId: "",
    source: "",
    localPath: "",
    base64: "",
    remotePath: "",
    contentType: "",
  },
} as NodeDefinition;

const definitions: DefinitionIndex = { [uploadDefinition.type]: uploadDefinition };

/* ------------------------------------------------------------------ */
/* harness                                                             */
/* ------------------------------------------------------------------ */

const apiStub: EditorApi = {
  secrets: [],
  pipelines: [],
  databases: [],
  storages: [
    { id: "stg-1", name: "Reports bucket", driver: "s3", createdAt: "2026-08-29T10:00:00Z", updatedAt: "2026-08-29T10:00:00Z" },
    { id: "stg-2", name: "Legacy FTP", driver: "ftp", createdAt: "2026-08-29T10:00:00Z", updatedAt: "2026-08-29T10:00:00Z" },
  ] as never,
  identities: [],
  discordIdentities: [],
  telegramIdentities: [],
  validateJavaScript: async () => undefined,
  generateCode: async () => ({ code: "", explanation: "" }) as never,
  inspectDatabase: async () => ({ id: "", name: "", tables: [] }) as never,
  debugDatabase: async () => ({ columns: [], rows: [], rowCount: 0, durationMs: 0 }) as never,
  openDocs: () => undefined,
  executions: [],
  onLoadExecution: () => undefined,
};

function App() {
  const [values, setValues] = useState<Record<string, unknown>>(() =>
    structuredClone(uploadDefinition.defaultConfig ?? {}),
  );

  const baseNode: GraphNode = {
    id: "upload-1",
    type: uploadDefinition.type,
    title: uploadDefinition.label,
    icon: "cloud",
    group: "Storage",
    summary: uploadDefinition.description ?? "",
    x: 0,
    y: 0,
    status: "idle",
    inputs: [],
    outputs: [],
    fields: [],
    values,
  };
  const node = useMemo(() => refreshNode(baseNode, definitions), [values]);

  return (
    <div className="flex h-screen w-screen flex-col bg-ink-900 text-fg">
      <div className="flex h-11 shrink-0 items-center gap-2 border-b border-seam px-3">
        <span className="text-[11px] font-medium tracking-wide text-fg-faint uppercase">Storage</span>
        <span className="rounded-md bg-ink-750 px-2.5 py-1 text-[12px] font-medium">Upload File</span>
        <span className="ml-auto text-[11px] text-fg-faint">
          switching the Source dropdown gates both the inspector fields and the canvas pins
        </span>
      </div>
      <div className="flex min-h-0 flex-1">
        <div className="w-[300px] shrink-0 overflow-y-auto border-r border-seam p-3">
          <p className="mb-2 text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">Input pins on canvas</p>
          <ul className="space-y-1.5" data-harness-pins>
            {node.inputs.map((input) => (
              <li key={input.id} className="flex items-center gap-2 text-[12px] text-fg-subtle">
                <span
                  className={`h-[7px] w-[7px] shrink-0 border border-ink-500 ${
                    input.kind === "exec" ? "rounded-[1px] bg-ink-300" : "rounded-full"
                  }`}
                  style={input.kind === "data" ? { background: input.color } : undefined}
                />
                <span className="truncate">{input.label}</span>
                <span className="ml-auto font-mono text-[10px] text-fg-faint">{input.id}</span>
              </li>
            ))}
          </ul>
        </div>
        <div className="w-[400px] shrink-0 overflow-y-auto">
          <Inspector
            node={node}
            log={[] as LogEntry[]}
            api={apiStub}
            onChange={(key, value) => setValues((current) => ({ ...current, [key]: value }))}
          />
        </div>
        <div className="flex-1 p-4">
          <p className="text-[12px] text-fg-faint">
            Live values:
          </p>
          <pre className="mt-2 max-w-full overflow-x-auto rounded-md border border-ink-700/70 bg-ink-850 p-2.5 font-mono text-[11px] leading-relaxed text-fg-muted">
{JSON.stringify(values, null, 2)}
          </pre>
        </div>
      </div>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
