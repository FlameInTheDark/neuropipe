/* Verification for the dynamic document value pins.
   Part 1 pins the Go contract: the shared dynpins package (namespaced pin IDs,
   blank-row tolerance, duplicate guards, row cap), the five resolvers that
   mint pins from config rows (word valuePins, excel cellPins/columnPins/
   fieldPins, read-cell output pins), cell-reference validation at resolve
   time, and the merge semantics (wired overrides object, literal fills gaps).
   Part 2 pins the frontend twins: document-pins.ts parse/build/port helpers,
   the resolver cases in blueprint-dynamic-pins.ts for all five node types,
   the PinBindingsEditor wiring in the Inspector for both config field kinds,
   and the complex-kind registration in adapters.ts.
   Part 3 checks the i18n keys ship in all four locales and the node-catalog
   terms translate the four field labels in de/fr/ru.
   Run: npx tsx scripts/verify-document-pins.mts */

import { readFileSync } from "node:fs";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

const dynpins = readFileSync(new URL("../../internal/nodes/documents/dynpins/dynpins.go", import.meta.url), "utf8");
const docxNodes = readFileSync(new URL("../../internal/nodes/documents/docx/nodes.go", import.meta.url), "utf8");
const excelPackage = readFileSync(new URL("../../internal/nodes/documents/excel/package.go", import.meta.url), "utf8");
const excelNodes = readFileSync(new URL("../../internal/nodes/documents/excel/nodes.go", import.meta.url), "utf8");
const docPins = readFileSync(new URL("../src/lib/document-pins.ts", import.meta.url), "utf8");
const dynamicPins = readFileSync(new URL("../src/lib/blueprint-dynamic-pins.ts", import.meta.url), "utf8");
const inspector = readFileSync(new URL("../src/components/Inspector.tsx", import.meta.url), "utf8");
const editor = readFileSync(new URL("../src/components/PinBindingsEditor.tsx", import.meta.url), "utf8");
const adapters = readFileSync(new URL("../src/lib/adapters.ts", import.meta.url), "utf8");
const en = readFileSync(new URL("../src/i18n/en.ts", import.meta.url), "utf8");
const de = readFileSync(new URL("../src/i18n/de.ts", import.meta.url), "utf8");
const fr = readFileSync(new URL("../src/i18n/fr.ts", import.meta.url), "utf8");
const ru = readFileSync(new URL("../src/i18n/ru.ts", import.meta.url), "utf8");
const nodeCatalog = readFileSync(new URL("../src/lib/node-catalog.ts", import.meta.url), "utf8");

/* ------------------------------------------------------------------ */
/* Part 1: Go contract                                                 */
/* ------------------------------------------------------------------ */

/* the shared dynpins package exists with the namespaced pin IDs */
check("dynpins: pin IDs are namespaced with pin_", dynpins.includes('func PinID(rowID string) string { return "pin_" + rowID }'));
check("dynpins: blank rows are dropped as editor noise", dynpins.includes("Blank rows are mid-edit editor state"));
check("dynpins: duplicate pin IDs are rejected", dynpins.includes("duplicate pin ID"));
check("dynpins: duplicate names are rejected", dynpins.includes("duplicate name"));
check("dynpins: hand-written pin_ prefixed IDs are rejected", dynpins.includes(`remove the %q prefix`, "pin prefix guard"));
check("dynpins: row cap keeps the editor responsive", dynpins.includes("MaxRows = 32"));
check("dynpins: input pins carry the row literal as default", dynpins.includes("MaxConnections: 1, Default: row.Value"));
check("dynpins: WiredValues only reports actually connected pins", dynpins.includes("invocation.ConnectedInputs[PinID(row.ID)]"));
check("dynpins: FallbackLiterals feed merge-gap filling", dynpins.includes("apply them only to names the primary object does not define"));

/* Word: value pins resolver + merge semantics + relaxed required */
check("word: template fill registers a resolver", docxNodes.includes("registerResolved(registrar, templateFillDefinition(), resolveTemplateFill, executeTemplateFill)"));
check("word: valuePins config field uses the pin-bindings editor", docxNodes.includes(`{Name: "valuePins", Label: "Value pins", Kind: "pin-bindings"`));
check("word: resolver expands value pins into inputs", docxNodes.includes('dynpins.Configured(configOf(node), "valuePins")'));
check("word: values object pin is optional now", /objectPin\("values", "Values", domain\.PinInput, false\)/.test(docxNodes));
check("word: wired pins override the values object", docxNodes.includes("dynpins.WiredValues(invocation, rows)"));
check("word: literals only fill gaps the object leaves open", docxNodes.includes("dynpins.FallbackLiterals(rows)"));
check("word: combined empty state keeps an actionable error", docxNodes.includes("at least one placeholder value or value pin is required"));

/* Excel: four node resolvers + cell validation */
check("excel: write cell resolver validates references", excelNodes.includes('cellReference("cell pin", row.Name)'));
check("excel: write cell pins come from cellPins config", /resolveWriteCell[\s\S]*?Configured\(configOf\(node\), "cellPins"\)/.test(excelNodes));
check("excel: read cell resolver mints output pins", /resolveReadCell[\s\S]*?dynpins\.OutputPins\(rows, "#a1a1aa"\)/.test(excelNodes));
check("excel: read cell single cell is optional", /textPin\("cell", "Cell", domain\.PinInput, false\)/.test(excelNodes));
check("excel: read cell pins read through the node's value mode", /raw := configured\(invocation, "valueMode"\) != "formatted"/.test(excelNodes));
check("excel: append rows resolver expands column pins", /resolveAppendRows[\s\S]*?Configured\(configOf\(node\), "columnPins"\)/.test(excelNodes));
check("excel: append pin row joins the rows input", excelNodes.includes("columnValues := dynpins.Values(invocation, columnRows)"));
check("excel: rows pin is optional now", /anyPin\("rows", "Rows", domain\.PinInput, false\)/.test(excelNodes));
check("excel: update row resolver expands field pins", /resolveUpdateRow[\s\S]*?Configured\(configOf\(node\), "fieldPins"\)/.test(excelNodes));
check("excel: update fields merge wired pins over the object", excelPackage.includes("dynpins.WiredValues(invocation, rows)"));
check("excel: update fields literal gap filling", excelPackage.includes("dynpins.FallbackLiterals(rows)"));
check("excel: blank rows JSON no longer errors before pins", excelPackage.includes("column pins may carry the row"));

/* ------------------------------------------------------------------ */
/* Part 2: Frontend twins                                              */
/* ------------------------------------------------------------------ */

check("fe: document-pins parses the same row contract", docPins.includes("parsePinBindings"));
check("fe: input ports mirror the Go InputPins (pin_ namespace + defaults)", docPins.includes("id: `pin_${row.id}`"));
check("fe: output ports mirror the Go OutputPins", docPins.includes("pinBindingOutputPorts"));
check("fe: blank rows are dropped on build", docPins.includes('row.name !== ""'));
check("fe: resolver twin handles word template fill", dynamicPins.includes('"action:word_template_fill"'));
check("fe: resolver twin handles excel write cell", dynamicPins.includes('"action:excel_write_cell"'));
check("fe: resolver twin handles excel append rows", dynamicPins.includes('"action:excel_append_rows"'));
check("fe: resolver twin handles excel update row", dynamicPins.includes('"action:excel_update_row"'));
check("fe: resolver twin mints read-cell output pins", dynamicPins.includes('"action:excel_read_cell"'));
check("fe: twins read the per-node config key", dynamicPins.includes("documentPinsKey(definition.type)"));
check("fe: inspector renders the input binding editor", inspector.includes('field.kind === "pin-bindings"'));
check("fe: inspector renders the output binding editor", inspector.includes('field.kind === "pin-bindings-output"'));
check("fe: PinBindingsEditor adapts to output mode", editor.includes('mode: "input" | "output"'));
check("fe: PinBindingsEditor reuses the draft-row round-trip", editor.includes("useDraftRows"));
check("fe: adapters register both complex kinds", adapters.includes('"pin-bindings",') && adapters.includes('"pin-bindings-output",'));

/* ------------------------------------------------------------------ */
/* Part 3: i18n + catalog terms                                        */
/* ------------------------------------------------------------------ */

const i18nFiles: Record<string, string> = { en, de, fr, ru };
const editorKeys = [
  "pinNameColumn",
  "pinLabelPlaceholder",
  "pinValuePlaceholder",
  "pinNoRows",
  "pinAddRow",
  "pinAddOutputRow",
];
for (const [locale, source] of Object.entries(i18nFiles)) {
  for (const key of editorKeys) {
    check(`i18n ${locale}: editor.${key}`, source.includes(`${key}: `), "key missing");
  }
}
for (const locale of ["de", "fr", "ru"]) {
  for (const term of ["Value pins", "Cell pins", "Column pins", "Field pins"]) {
    check(`catalog ${locale}: term ${term}`, nodeCatalog.includes(`'${term}':`), "term missing");
  }
}

/* ------------------------------------------------------------------ */
/* Part 4: runtime twin behavior (real module imports)                 */
/* ------------------------------------------------------------------ */

const { parsePinBindings, pinBindingInputPorts, pinBindingOutputPorts, buildPinBindingsPayload, nextPinBindingID } =
  await import("../src/lib/document-pins");
const { resolveConfigDrivenInputs, resolveConfigDrivenOutputs } = await import("../src/lib/blueprint-dynamic-pins");

const wordDefinition = {
  type: "action:word_template_fill",
  inputs: [
    { id: "in", label: "Exec", kind: "exec", direction: "input" },
    { id: "templatePath", label: "Template", kind: "data", direction: "input" },
  ],
  outputs: [],
  defaultConfig: { valuePins: [] },
} as never;
const fillConfig = {
  valuePins: [
    { id: "field_1", name: "customer", label: "Customer", value: "Contoso" },
    { id: "field_2", name: "amount", value: "" },
    { name: "unlabeled" },
  ],
};
const rows = parsePinBindings(fillConfig.valuePins);
check("twin: parses three rows, generates the missing id", rows.length === 3 && rows[2].id === "row_3");
check("twin: label falls back to the name", rows[1].label === "amount");
const inputPorts = pinBindingInputPorts(rows);
check(
  "twin: input ports use the pin_ namespace and carry literals as defaults",
  inputPorts[0].id === "pin_field_1" && inputPorts[0].default === "Contoso" && inputPorts[1].default === undefined,
);
check(
  "twin: word node resolves static plus dynamic inputs",
  resolveConfigDrivenInputs(wordDefinition, fillConfig).map((pin) => pin.id).join(",") ===
    "in,templatePath,pin_field_1,pin_field_2,pin_row_3",
);
check(
  "twin: read cell node resolves output pins per configured cell",
  (() => {
    const readDefinition = {
      type: "action:excel_read_cell",
      inputs: [],
      outputs: [{ id: "value", label: "Value", kind: "data", direction: "output" }],
      defaultConfig: { cellPins: [] },
    } as never;
    const outputs = resolveConfigDrivenOutputs(readDefinition, { cellPins: [{ id: "field_1", name: "B4", label: "Total" }] });
    return outputs.length === 2 && outputs[1].id === "pin_field_1" && outputs[1].label === "Total";
  })(),
);
check(
  "twin: output ports carry no defaults",
  pinBindingOutputPorts(rows).every((pin) => pin.default === undefined),
);
check(
  "twin: payload drops blank rows",
  JSON.stringify(buildPinBindingsPayload([{ id: "x", name: "", label: "", value: "" }, { id: "y", name: "ok", label: "", value: "1" }])) ===
    JSON.stringify([{ id: "y", name: "ok", label: "", value: "1" }]),
);
check(
  "twin: next id avoids collisions with used ids",
  nextPinBindingID([{ id: "field_1" }, { id: "field_2" }]) === "field_3" && nextPinBindingID([{ id: "field_1" }, { id: "field_3" }]) === "field_4",
);

/* summary */
console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
