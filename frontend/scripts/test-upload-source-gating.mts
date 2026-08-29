/* Unit tests for the storage Upload File source gating: the visibleWhen
   predicate engine (adapters) and the config-driven pin filtering twin
   (blueprint-dynamic-pins) for the merged upload node, plus backend wiring
   assertions so a refactor cannot silently drop the dropdown.
   Run: npx tsx scripts/test-upload-source-gating.mts */

import "./dom-stub.mts";
import { visibleFields } from "../src/lib/adapters";
import type { ConfigField } from "../src/lib/types";
import { resolveConfigDrivenInputs } from "../src/lib/blueprint-dynamic-pins";
import type { NodeDefinition, NodePort } from "../src/lib/types";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

/* ------------------------------------------------------------------ */
/* visibleWhen predicate semantics (upload field set)                  */
/* ------------------------------------------------------------------ */

const field = (name: string, visibleWhen?: string): ConfigField => ({
  name,
  label: name,
  kind: "string",
  visibleWhen,
});

// Mirrors the fields the Go definition declares (storageId prepended by
// Definition() is never gated).
const uploadFields: ConfigField[] = [
  field("source"),
  field("localPath", "source=file|source="),
  field("base64", "source=base64|source="),
  field("remotePath"),
  field("contentType"),
];

const keys = (values: Record<string, unknown>) =>
  visibleFields(uploadFields, values).map((f) => f.name);

check(
  "auto mode (explicit empty) shows localPath + base64 fields",
  JSON.stringify(keys({ source: "" })) ===
    JSON.stringify(["source", "localPath", "base64", "remotePath", "contentType"]),
);
check(
  "legacy graph (missing key) reads as auto",
  JSON.stringify(keys({})) ===
    JSON.stringify(["source", "localPath", "base64", "remotePath", "contentType"]),
);
check(
  "disk mode shows only the local path field",
  JSON.stringify(keys({ source: "file" })) ===
    JSON.stringify(["source", "localPath", "remotePath", "contentType"]),
);
check(
  "node mode hides both disk and base64 fields",
  JSON.stringify(keys({ source: "bytes" })) ===
    JSON.stringify(["source", "remotePath", "contentType"]),
);
check(
  "base64 mode shows only the base64 field",
  JSON.stringify(keys({ source: "base64" })) ===
    JSON.stringify(["source", "base64", "remotePath", "contentType"]),
);
check(
  // Mirrors the send-node semantics: a typo hides the gated fields (no OR
  // term matches) while execution and pins fall back to Auto.
  "unknown mode value hides the gated fields",
  JSON.stringify(keys({ source: "url" })) ===
    JSON.stringify(["source", "remotePath", "contentType"]),
);

/* ------------------------------------------------------------------ */
/* pin filtering twin                                                  */
/* ------------------------------------------------------------------ */

const pin = (id: string): NodePort => ({
  id,
  label: id,
  kind: "data",
  direction: "input",
  dataType: "text",
  color: "#fff",
  maxConnections: 1,
});

// Mirrors the Go uploadFileDefinition() input ports.
const uploadDefinition: NodeDefinition = {
  type: "action:storage_upload_file",
  label: "Upload File",
  category: "Storage",
  inputs: [
    { ...pin("in"), kind: "exec" },
    pin("localPath"),
    { ...pin("data"), dataType: "bytes" },
    pin("base64"),
    pin("remotePath"),
    pin("contentType"),
  ],
  outputs: [],
  fields: [],
  defaultConfig: { source: "", localPath: "", base64: "", remotePath: "", contentType: "" },
};

const ids = (config: Record<string, unknown>) =>
  resolveConfigDrivenInputs(uploadDefinition, config).map((p) => p.id);

const uploadAll = ["in", "localPath", "data", "base64", "remotePath", "contentType"];
check(
  "auto keeps every pin",
  JSON.stringify(ids({})) === JSON.stringify(uploadAll),
  ids({}).join(","),
);
check(
  "disk mode keeps localPath only",
  JSON.stringify(ids({ source: "file" })) ===
    JSON.stringify(["in", "localPath", "remotePath", "contentType"]),
  ids({ source: "file" }).join(","),
);
check(
  "node mode keeps the data pin only",
  JSON.stringify(ids({ source: "bytes" })) ===
    JSON.stringify(["in", "data", "remotePath", "contentType"]),
  ids({ source: "bytes" }).join(","),
);
check(
  "base64 mode keeps the base64 pin only",
  JSON.stringify(ids({ source: "base64" })) ===
    JSON.stringify(["in", "base64", "remotePath", "contentType"]),
  ids({ source: "base64" }).join(","),
);
check(
  "default config (new node) resolves as auto",
  JSON.stringify(ids(uploadDefinition.defaultConfig as Record<string, unknown>)) ===
    JSON.stringify(uploadAll),
);
check(
  "unknown mode value keeps every pin (typo safety)",
  JSON.stringify(ids({ source: "wat" })) === JSON.stringify(uploadAll),
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);

/* ------------------------------------------------------------------ */
/* part 2: pin the backend wiring so a refactor cannot drop it         */
/* ------------------------------------------------------------------ */

import { readFileSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..", "..");
const go = (relative: string) => readFileSync(path.join(root, relative), "utf8");

function pins(name: string, ok: boolean) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}`);
  if (!ok) process.exitCode = 1;
}

const transfer = go("internal/nodes/storage/transfer.go");
pins(
  "upload node declares the source select with the four options",
  transfer.includes(`{Name: "source", Label: "Source", Kind: "select", Options: uploadSourceOptions()}`) &&
    transfer.includes(`{Value: uploadSourceDisk, Label: "From disk"}`) &&
    transfer.includes(`{Value: uploadSourceNode, Label: "From node"}`) &&
    transfer.includes(`{Value: uploadSourceBase64, Label: "From base64"}`),
);
pins(
  "upload node gates localPath/base64 fields with visibleWhen",
  transfer.includes(`VisibleWhen: "source=" + uploadSourceDisk + "|source="`) &&
    transfer.includes(`VisibleWhen: "source=" + uploadSourceBase64 + "|source="`),
);
pins(
  "upload node declares all three source pins",
  transfer.includes(`Text("localPath", "Local path", domain.PinInput, false)`) &&
    transfer.includes(`Bytes("data", "Data", domain.PinInput, false)`) &&
    transfer.includes(`Text("base64", "Base64", domain.PinInput, false)`),
);
pins(
  "upload node registers a resolver",
  transfer.includes("Resolver: resolveUpload, Executor: executeUploadFile") ||
    go("internal/nodes/storage/manage.go").includes("Resolver: resolveUpload, Executor: executeUploadFile"),
);
pins(
  "legacy Upload Data node is folded into the upload node",
  !go("internal/nodes/storage/manage.go").includes("uploadDataDefinition"),
);
pins(
  "auto mode falls back disk -> bytes -> base64 in order",
  transfer.includes("default: // Auto: disk file, then node bytes, then base64"),
);
pins(
  "base64 text decoding handles data URLs",
  go("internal/nodes/storage/storage.go").includes("func Base64Text(text string) ([]byte, error)"),
);
