/**
 * Send-source live entry — the REAL Inspector component rendered over the
 * REAL adapter pipeline (visibleFields + fieldDefFromConfig + refreshNode)
 * with the send-node definitions mirrored from the Go packages: Discord Send
 * Message, Telegram Send Photo, and Telegram Send Document. Switching the
 * Image/Photo/Document source dropdown must show/hide the matching config
 * fields (inspector) and input pins (left panel) live. The harness script
 * switches nodes and dropdown values, then screenshots each state.
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
/* definitions mirrored from the Go packages                           */
/* ------------------------------------------------------------------ */

const sourceOptions = [
  { value: "", label: "Auto — use whatever is set" },
  { value: "url", label: "URL" },
  { value: "file", label: "Local file" },
  { value: "base64", label: "Base64" },
  { value: "bytes", label: "Bytes from another node" },
];

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

const discordDefinition: NodeDefinition = {
  type: "action:discord_send_message",
  label: "Send Discord Message",
  category: "Discord",
  icon: "send",
  color: "#5865f2",
  mode: "impure",
  inputs: [
    { id: "in", label: "Send", kind: "exec", direction: "input", color: "#fafafa", maxConnections: 1 },
    pin("message", "Message"),
    pin("channel", "Channel ID"),
    pin("replyToMessageId", "Reply to message ID"),
    pin("embedsJson", "Embeds JSON"),
    pin("fileUrl", "File URL"),
    pin("filePath", "File path"),
    pin("fileBase64", "File base64"),
    pin("fileData", "File data", "any"),
    pin("fileName", "File name"),
    pin("identityId", "Identity"),
  ],
  outputs: [],
  fields: [
    field({ name: "identityId", label: "Bot identity", kind: "discord-identity" }),
    field({ name: "channel", label: "Channel ID", placeholder: "123456789012345678", required: true }),
    field({ name: "message", label: "Message", kind: "textarea" }),
    field({ name: "embeds", label: "Embeds", kind: "embed-editor" }),
    field({ name: "embedsJson", label: "Embeds JSON", kind: "textarea", placeholder: `[{"title":"Hello"}] — overrides the embed editor` }),
    field({ name: "imageSource", label: "Image source", kind: "select", options: sourceOptions }),
    field({ name: "fileUrl", label: "File URL", placeholder: "https://example.com/report.pdf", visibleWhen: "imageSource=url|imageSource=" }),
    field({ name: "filePath", label: "File path", placeholder: "C:\\Reports\\report.pdf", visibleWhen: "imageSource=file|imageSource=" }),
    field({ name: "fileBase64", label: "File base64", kind: "textarea", placeholder: "aGVsbG8gd29ybGQ= or a data: URL", visibleWhen: "imageSource=base64|imageSource=" }),
    field({ name: "fileName", label: "File name", placeholder: "report.pdf — names the File data pin", visibleWhen: "imageSource=base64|imageSource=bytes|imageSource=" }),
  ],
  defaultConfig: {
    identityId: "",
    channel: "",
    message: "",
    embeds: { version: 1, pins: [], embeds: [] },
    embedsJson: "",
    imageSource: "",
    fileUrl: "",
    filePath: "",
    fileBase64: "",
    fileName: "",
  },
} as NodeDefinition;

const photoDefinition: NodeDefinition = {
  type: "action:telegram_send_photo",
  label: "Send Telegram Photo",
  category: "Telegram",
  icon: "camera",
  color: "#229ed9",
  mode: "impure",
  inputs: [
    { id: "in", label: "Send", kind: "exec", direction: "input", color: "#fafafa", maxConnections: 1 },
    pin("photoUrl", "Photo URL"),
    pin("photoPath", "Photo path"),
    pin("photoBase64", "Photo base64"),
    pin("photoData", "Photo data", "any"),
    pin("photoName", "Photo name"),
    pin("caption", "Caption"),
    pin("chatId", "Chat ID"),
    pin("parseMode", "Parse mode"),
    pin("identityId", "Identity"),
  ],
  outputs: [],
  fields: [
    field({ name: "identityId", label: "Bot identity", kind: "telegram-identity" }),
    field({ name: "photoSource", label: "Photo source", kind: "select", options: sourceOptions }),
    field({ name: "photoUrl", label: "Photo URL", placeholder: "https://example.com/photo.jpg — Telegram fetches this server-side", visibleWhen: "photoSource=url|photoSource=" }),
    field({ name: "photoPath", label: "Photo path", placeholder: "C:\\Pictures\\photo.jpg", visibleWhen: "photoSource=file|photoSource=" }),
    field({ name: "photoBase64", label: "Photo base64", kind: "textarea", placeholder: "aGVsbG8gd29ybGQ= or a data: URL", visibleWhen: "photoSource=base64|photoSource=" }),
    field({ name: "chatId", label: "Chat ID", placeholder: "123456 or @mychannel", required: true }),
    field({ name: "photoName", label: "Photo name", placeholder: "photo.jpg — names the Base64 and Bytes pins", visibleWhen: "photoSource=base64|photoSource=bytes|photoSource=" }),
    field({ name: "caption", label: "Caption", kind: "textarea" }),
    field({ name: "parseMode", label: "Parse mode", kind: "select", options: [
      { value: "", label: "Plain text" },
      { value: "HTML", label: "HTML" },
      { value: "MarkdownV2", label: "MarkdownV2" },
    ] }),
  ],
  defaultConfig: {
    identityId: "",
    photoSource: "",
    photoUrl: "",
    photoPath: "",
    photoBase64: "",
    photoName: "",
    chatId: "",
    caption: "",
    parseMode: "",
  },
} as NodeDefinition;

const documentDefinition: NodeDefinition = {
  type: "action:telegram_send_document",
  label: "Send Telegram Document",
  category: "Telegram",
  icon: "send",
  color: "#229ed9",
  mode: "impure",
  inputs: [
    { id: "in", label: "Send", kind: "exec", direction: "input", color: "#fafafa", maxConnections: 1 },
    pin("documentUrl", "Document URL"),
    pin("documentPath", "Document path"),
    pin("documentBase64", "Document base64"),
    pin("documentData", "Document data", "any"),
    pin("fileName", "File name"),
    pin("caption", "Caption"),
    pin("chatId", "Chat ID"),
    pin("parseMode", "Parse mode"),
    pin("replyToMessageId", "Reply to message ID"),
    pin("disableNotification", "Silent", "boolean"),
    pin("identityId", "Identity"),
  ],
  outputs: [],
  fields: [
    field({ name: "identityId", label: "Bot identity", kind: "telegram-identity" }),
    field({ name: "documentSource", label: "Document source", kind: "select", options: sourceOptions }),
    field({ name: "documentUrl", label: "Document URL", placeholder: "https://example.com/report.pdf — Telegram fetches this server-side", visibleWhen: "documentSource=url|documentSource=" }),
    field({ name: "documentPath", label: "Document path", placeholder: "C:\\Reports\\report.pdf", visibleWhen: "documentSource=file|documentSource=" }),
    field({ name: "documentBase64", label: "Document base64", kind: "textarea", placeholder: "aGVsbG8gd29ybGQ= or a data: URL", visibleWhen: "documentSource=base64|documentSource=" }),
    field({ name: "chatId", label: "Chat ID", placeholder: "123456 or @mychannel", required: true }),
    field({ name: "fileName", label: "File name", placeholder: "report.pdf — names the Base64 and Bytes pins", visibleWhen: "documentSource=base64|documentSource=bytes|documentSource=" }),
    field({ name: "caption", label: "Caption", kind: "textarea" }),
    field({ name: "parseMode", label: "Parse mode", kind: "select", options: [
      { value: "", label: "Plain text" },
      { value: "HTML", label: "HTML" },
      { value: "MarkdownV2", label: "MarkdownV2" },
    ] }),
    field({ name: "replyToMessageId", label: "Reply to message ID" }),
    field({ name: "disableNotification", label: "Silent (no notification)", kind: "boolean" }),
  ],
  defaultConfig: {
    identityId: "",
    documentSource: "",
    documentUrl: "",
    documentPath: "",
    documentBase64: "",
    fileName: "",
    chatId: "",
    caption: "",
    parseMode: "",
    replyToMessageId: "",
    disableNotification: false,
  },
} as NodeDefinition;

const definitions: DefinitionIndex = {
  [discordDefinition.type]: discordDefinition,
  [photoDefinition.type]: photoDefinition,
  [documentDefinition.type]: documentDefinition,
};

/* ------------------------------------------------------------------ */
/* harness                                                             */
/* ------------------------------------------------------------------ */

const apiStub: EditorApi = {
  secrets: [],
  pipelines: [],
  databases: [],
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
  const [type, setType] = useState<string>(discordDefinition.type);
  const [values, setValues] = useState<Record<string, unknown>>(() =>
    structuredClone(definitions[discordDefinition.type].defaultConfig ?? {}),
  );

  const definition = definitions[type];
  const baseNode: GraphNode = {
    id: `${type}-1`,
    type,
    title: definition.label,
    icon: definition.icon ?? "Boxes",
    group: definition.category ?? "",
    summary: definition.description ?? "",
    x: 0,
    y: 0,
    status: "idle",
    inputs: [],
    outputs: [],
    fields: [],
    values,
  };
  const node = useMemo(() => refreshNode(baseNode, definitions), [type, values]);

  const switchNode = (next: string) => {
    setType(next);
    setValues(structuredClone(definitions[next].defaultConfig ?? {}));
  };

  return (
    <div className="flex h-screen w-screen flex-col bg-ink-900 text-fg">
      <div className="flex h-11 shrink-0 items-center gap-2 border-b border-seam px-3">
        <span className="text-[11px] font-medium tracking-wide text-fg-faint uppercase">Send nodes</span>
        {[discordDefinition, photoDefinition, documentDefinition].map((definition) => (
          <button
            key={definition.type}
            data-harness-node={definition.type}
            onClick={() => switchNode(definition.type)}
            className={`rounded-md px-2.5 py-1 text-[12px] font-medium transition ${
              type === definition.type ? "bg-ink-750 text-fg" : "text-fg-subtle hover:bg-ink-850 hover:text-fg-muted"
            }`}
          >
            {definition.label}
          </button>
        ))}
        <span className="ml-auto text-[11px] text-fg-faint">
          switching the source dropdown gates both the inspector fields and the canvas pins
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
{JSON.stringify(
  Object.fromEntries(
    Object.entries(values).filter(([key]) => key !== "embeds"),
  ),
  null,
  2,
)}
          </pre>
        </div>
      </div>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
