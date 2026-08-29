/* Unit tests for the storage-select inspector wiring: the backend storage
   nodes declare kind "storage-select" on their storageId config field; the
   adapter must map it to a dynamic select whose options come from the
   storages list on the editor API. Also verifies the bridge surface has the
   full set of storage bindings.
   Run: npx tsx scripts/test-storage-select.mts */

import "./dom-stub.mts";
import { fieldDefFromConfig } from "../src/lib/adapters";
import type { ConfigField } from "../src/lib/types";
import { readFileSync } from "node:fs";
import path from "node:path";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

/* ------------------------------------------------------------------ */
/* storage-select field mapping                                        */
/* ------------------------------------------------------------------ */

const storageField: ConfigField = { name: "storageId", label: "Storage", kind: "storage-select", required: true };
const def = fieldDefFromConfig(storageField);

check("storage-select maps to a select field", def.type === "select", `got ${def.type}`);
check("storage-select select is dynamic", def.dynamic === "storages", `got ${def.dynamic}`);
check("field key survives mapping", def.key === "storageId", `got ${def.key}`);
check("required flag survives mapping", def.required === true);

const plainField = fieldDefFromConfig({ name: "path", label: "Path", kind: "string" });
check("plain string fields are not dynamic", plainField.dynamic === undefined && plainField.type === "text");

/* ------------------------------------------------------------------ */
/* dynamic option resolution (mirrors Inspector's optionMap)           */
/* ------------------------------------------------------------------ */

const storages = [
  { id: "stg-1", name: "Company Backups", driver: "s3" },
  { id: "stg-2", name: "Nightly FTP", driver: "ftp" },
] as const;

const optionMap: Record<string, Array<{ value: string; label: string }>> = {
  storages: storages.map((s) => ({ value: s.id, label: s.name })),
};
const options = optionMap[def.dynamic as string] ?? [];
check("two storage options resolve", options.length === 2);
check("option values are storage ids", options[0].value === "stg-1" && options[1].value === "stg-2");
check("option labels are storage names", options[0].label === "Company Backups" && options[1].label === "Nightly FTP");

/* ------------------------------------------------------------------ */
/* bridge surface (source assertion — importing the binding would pull  */
/* the Wails runtime, which needs a real browser)                      */
/* ------------------------------------------------------------------ */

const bindings = readFileSync(
  path.resolve(import.meta.dirname, "../bindings/neuropipe/desktop.ts"),
  "utf8",
);
const requiredMethods = [
  "ListStorages",
  "RegisterStorage",
  "UpdateStorage",
  "DeleteStorage",
  "PingStorage",
  "TestStorage",
  "StorageListFiles",
  "StorageUploadFile",
  "StorageDownloadFile",
  "StorageDeleteEntry",
  "StorageMakeDir",
  "StorageMoveEntry",
  "ChooseStorageUploadFile",
  "ChooseStorageSaveFile",
];
for (const method of requiredMethods) {
  check(
    `desktop binding calls ${method}`,
    bindings.includes(`"${method}"`),
    "method name not found in bindings/neuropipe/desktop.ts",
  );
}
check("bindings import Storage type", bindings.includes("Storage,\n"));
check("bindings import SaveStorageRequest type", bindings.includes("SaveStorageRequest,"));

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
