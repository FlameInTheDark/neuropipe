import { dataPinColor } from "@/lib/node-pins";
import { normalizeDrawImageDoc } from "@/lib/draw-image";
import { normalizeEmbedDoc } from "@/lib/embed";
import {
  resolveJavaScriptInputs,
  resolveJavaScriptOutputs,
} from "@/lib/javascript-node";
import { typeSpecFromDataType } from "@/lib/type-spec";
import type { DataType, FormItemValue, FormLayoutValue, NodeDefinition, NodePort, TypeSpec } from "@/lib/types";

export interface FieldOutputValue {
  id: string;
  label: string;
  path: string;
  dataType: DataType;
}

export interface ObjectFieldValue {
  id: string;
  label: string;
  key: string;
  dataType: DataType;
}

export type HtmlReturnMode = "text" | "html" | "attribute";

export interface HtmlExtractionValue {
  id: string;
  label: string;
  selector: string;
  mode: HtmlReturnMode;
  attribute: string;
  returnAll: boolean;
}

export type SwitchComparator =
  | "equals"
  | "not_equals"
  | "contains"
  | "starts_with"
  | "ends_with"
  | "greater_than"
  | "greater_than_or_equal"
  | "less_than"
  | "less_than_or_equal";

export interface SwitchCaseValue {
  id: string;
  label: string;
  valueType: Extract<DataType, "text" | "number" | "boolean">;
  value: string | number | boolean;
}

export interface SwitchConfigValue {
  comparator: SwitchComparator;
  cases: SwitchCaseValue[];
}

export interface RouteOptionValue {
  id: string;
  label: string;
  description: string;
}

const dataTypes: readonly DataType[] = [
  "any",
  "text",
  "number",
  "boolean",
  "object",
  "list",
  "bytes",
];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function parseStructuredValue(value: unknown): unknown {
  if (typeof value !== "string") return value;
  try {
    return JSON.parse(value);
  } catch {
    return undefined;
  }
}

function typeSpecFromValue(value: unknown): TypeSpec {
  const parsed = parseStructuredValue(value);
  if (!isRecord(parsed) || typeof parsed.kind !== "string") return { kind: "any" };
  return parsed as unknown as TypeSpec;
}

function dataTypeForTypeSpec(type: TypeSpec): DataType {
  switch (type.kind) {
    case "string": return "text";
    case "int":
    case "float": return "number";
    case "bool": return "boolean";
    case "list": return "list";
    case "bytes": return "bytes";
    case "map":
    case "record": return "object";
    default: return "any";
  }
}

/** Cast targets cover the concrete data types; the Go resolver is the twin. */
function castTargetDataType(target: unknown): DataType {
  switch (target) {
    case "text":
    case "number":
    case "boolean":
    case "object":
    case "list":
    case "bytes":
      return target;
    default:
      return "any";
  }
}

/** Objects cast into the graph-wide map<string, any> shape so the output
 *  connects to first-party object inputs (KV hash fields, SQL rows, storage). */
function castTypeSpec(dataType: DataType): TypeSpec {
  if (dataType === "object") {
    return { kind: "map", key: { kind: "string" }, value: { kind: "any" } };
  }
  return typeSpecFromDataType(dataType);
}

type TextBytesRepresentation = "text" | "bytes";

function textBytesRepresentation(value: unknown): TextBytesRepresentation {
        return value === "text" ? "text" : "bytes";
}

function textBytesPin(pin: NodePort, representation: TextBytesRepresentation): NodePort {
        const dataType: DataType = representation === "text" ? "text" : "bytes";
        return {
                ...pin,
                dataType,
                type: representation === "text" ? { kind: "string" } : { kind: "bytes" },
                color: dataPinColor(dataType),
        };
}

export function isDataType(value: unknown): value is DataType {
  return dataTypes.some((dataType) => dataType === value);
}

export function isSwitchComparator(value: unknown): value is SwitchComparator {
  return [
    "equals",
    "not_equals",
    "contains",
    "starts_with",
    "ends_with",
    "greater_than",
    "greater_than_or_equal",
    "less_than",
    "less_than_or_equal",
  ].some((comparator) => comparator === value);
}

function isSwitchValueType(
  value: unknown,
): value is SwitchCaseValue["valueType"] {
  return value === "text" || value === "number" || value === "boolean";
}

export function switchConfigFromValue(
  value: unknown,
  legacyOptions?: unknown,
): SwitchConfigValue {
  const parsed = parseStructuredValue(value);
  if (!isRecord(parsed)) {
    const legacyCases = routeOptionsFromValue(legacyOptions).map((item) => ({
      id: item.id,
      label: item.label,
      valueType: "text" as const,
      value: item.id,
    }));
    return { comparator: "equals", cases: legacyCases };
  }
  const comparator = isSwitchComparator(parsed.comparator)
    ? parsed.comparator
    : "equals";
  const items = Array.isArray(parsed.cases) ? parsed.cases : [];
  const seen = new Set<string>();
  return {
    comparator,
    cases: items.flatMap((item) => {
      if (!isRecord(item) || typeof item.id !== "string") return [];
      const id = item.id.trim();
      if (!id || seen.has(id)) return [];
      seen.add(id);
      const valueType = isSwitchValueType(item.valueType)
        ? item.valueType
        : "text";
      const rawValue = item.value;
      const valueForType =
        valueType === "number"
          ? typeof rawValue === "number"
            ? rawValue
            : ""
          : valueType === "boolean"
            ? typeof rawValue === "boolean"
              ? rawValue
              : false
            : typeof rawValue === "string"
              ? rawValue
              : "";
      return [{
        id,
        label:
          typeof item.label === "string" && item.label.trim()
            ? item.label.trim()
            : id,
        valueType,
        value: valueForType,
      }];
    }),
  };
}

export function routeOptionsFromValue(value: unknown): RouteOptionValue[] {
  const parsed = parseStructuredValue(value);
  if (!Array.isArray(parsed)) return [];
  return parsed.flatMap((item) => {
    if (!isRecord(item) || typeof item.id !== "string") return [];
    const id = item.id.trim();
    if (!id) return [];
    return [{
      id,
      label:
        typeof item.label === "string" && item.label.trim()
          ? item.label
          : id,
      description: typeof item.description === "string" ? item.description : "",
    }];
  });
}

export function fieldOutputsFromValue(
  value: unknown,
  config?: Record<string, unknown>,
): FieldOutputValue[] {
  if (value === undefined && typeof config?.path === "string") {
    return [{ id: "value", label: "Value", path: config.path, dataType: "any" }];
  }
  const parsed = parseStructuredValue(value);
  if (!Array.isArray(parsed)) return [];
  const seen = new Set<string>();
  return parsed.flatMap((item, index) => {
    if (!isRecord(item) || typeof item.id !== "string") return [];
    const id = item.id.trim();
    if (!id || seen.has(id)) return [];
    seen.add(id);
    return [{
      id,
      label:
        typeof item.label === "string" && item.label.trim()
          ? item.label.trim()
          : `Value ${index + 1}`,
      path: typeof item.path === "string" ? item.path : "",
      dataType: isDataType(item.dataType) ? item.dataType : "any",
    }];
  });
}

export function objectFieldsFromValue(value: unknown): ObjectFieldValue[] {
  const parsed = parseStructuredValue(value);
  if (!Array.isArray(parsed)) return [];
  const seen = new Set<string>();
  return parsed.flatMap((item, index) => {
    if (!isRecord(item) || typeof item.id !== "string") return [];
    const id = item.id.trim();
    if (!id || seen.has(id)) return [];
    seen.add(id);
    return [{
      id,
      label:
        typeof item.label === "string" && item.label.trim()
          ? item.label.trim()
          : `Value ${index + 1}`,
      key: typeof item.key === "string" ? item.key : "",
      dataType: isDataType(item.dataType) ? item.dataType : "any",
    }];
  });
}

function isHtmlReturnMode(value: unknown): value is HtmlReturnMode {
  return value === "text" || value === "html" || value === "attribute";
}

export function htmlExtractionsFromValue(value: unknown): HtmlExtractionValue[] {
  const parsed = parseStructuredValue(value);
  if (!Array.isArray(parsed)) return [];
  const seen = new Set<string>();
  return parsed.flatMap((item, index) => {
    if (!isRecord(item) || typeof item.id !== "string") return [];
    const id = item.id.trim();
    if (!id || seen.has(id)) return [];
    seen.add(id);
    return [{
      id,
      label:
        typeof item.label === "string" && item.label.trim()
          ? item.label.trim()
          : `Value ${index + 1}`,
      selector: typeof item.selector === "string" ? item.selector : "",
      mode: isHtmlReturnMode(item.mode) ? item.mode : "text",
      attribute: typeof item.attribute === "string" ? item.attribute : "",
      returnAll: item.returnAll === true,
    }];
  });
}

export function nextMappingID<T extends { id: string }>(mappings: readonly T[]) {
  const used = new Set(mappings.map((mapping) => mapping.id));
  for (let index = mappings.length + 1; ; index += 1) {
    const id = `field_${index}`;
    if (!used.has(id)) return id;
  }
}

function definitionPorts(ports: NodePort[] | undefined | null): NodePort[] {
  return Array.isArray(ports) ? ports : [];
}

export function resolveConfigDrivenInputs(
  definition: NodeDefinition | undefined,
  config: Record<string, unknown>,
): NodePort[] {
  if (!definition) return [];
  const inputs = definitionPorts(definition.inputs);
  if (definition.type === "action:draw_image") {
    // dynamic input pins mirror the document's declared pins (Go resolver twin)
    const doc = normalizeDrawImageDoc(config.document ?? definition.defaultConfig?.document);
    const declared: NodePort[] = doc.pins.map((pin) => ({
      id: pin.name,
      label: pin.name,
      kind: "data",
      direction: "input",
      dataType: drawPinDataType(pin.type),
      type: drawPinTypeSpec(pin.type),
      color: dataPinColor(drawPinDataType(pin.type)),
      maxConnections: 1,
      default: pin.type === "text" && pin.default !== "" ? pin.default : undefined,
    }));
    return [...inputs, ...declared];
  }
  if (definition.type === "action:discord_send_message") {
    // dynamic input pins mirror the embed document's variables (Go resolver twin)
    const doc = normalizeEmbedDoc(config.embeds ?? definition.defaultConfig?.embeds);
    const declared: NodePort[] = doc.pins.map((pin) => ({
      id: pin.name,
      label: pin.name,
      kind: "data",
      direction: "input",
      dataType: drawPinDataType(pin.type),
      type: drawPinTypeSpec(pin.type),
      color: dataPinColor(drawPinDataType(pin.type)),
      maxConnections: 1,
      default: pin.type === "text" && pin.default !== "" ? pin.default : undefined,
    }));
    // the Image source dropdown gates the attachment pins (Go resolver twin)
    return filterSourcePins([...inputs, ...declared], imageSourceSpec, config.imageSource ?? definition.defaultConfig?.imageSource);
  }
  if (definition.type === "action:telegram_send_photo") {
    return filterSourcePins(inputs, photoSourceSpec, config.photoSource ?? definition.defaultConfig?.photoSource);
  }
  if (definition.type === "action:telegram_send_document") {
    return filterSourcePins(inputs, documentSourceSpec, config.documentSource ?? definition.defaultConfig?.documentSource);
  }
  if (definition.type === "action:storage_upload_file") {
    // the Source dropdown gates the upload pins (Go resolver twin)
    return filterUploadSourcePins(inputs, config.source ?? definition.defaultConfig?.source);
  }
  if (
    definition.type === "action:discord_reply_command" ||
    definition.type === "action:discord_followup_command" ||
    definition.type === "action:discord_edit_command_reply"
  ) {
    // dynamic input pins mirror the embed document's variables (Go resolver twin)
    const doc = normalizeEmbedDoc(config.embeds ?? definition.defaultConfig?.embeds);
    const declared: NodePort[] = doc.pins.map((pin) => ({
      id: pin.name,
      label: pin.name,
      kind: "data",
      direction: "input",
      dataType: drawPinDataType(pin.type),
      type: drawPinTypeSpec(pin.type),
      color: dataPinColor(drawPinDataType(pin.type)),
      maxConnections: 1,
      default: pin.type === "text" && pin.default !== "" ? pin.default : undefined,
    }));
    return [...inputs, ...declared];
  }
  if (definition.type === "action:javascript") {
    return resolveJavaScriptInputs({ ...definition, inputs }, config);
  }
  if (definition.type === "data:base64_encode" || definition.type === "data:base64_decode") {
    const representation = textBytesRepresentation(config.inputType ?? definition.defaultConfig?.inputType);
    return inputs.map((pin) =>
      pin.id === "value" ? textBytesPin(pin, representation) : pin,
    );
  }
  if (definition.type === "action:file_write") {
    const representation = textBytesRepresentation(config.contentType ?? definition.defaultConfig?.contentType);
    return inputs.map((pin) =>
      pin.id === "content" ? textBytesPin(pin, representation) : pin,
    );
  }
  if (
    definition.type === "llm:prompt" ||
    definition.type === "llm:extract" ||
    definition.type === "llm:boolean" ||
    definition.type === "llm:choice" ||
    definition.type === "llm:summarize" ||
    definition.type === "llm:agent" ||
    definition.type === "llm:coding_agent"
  ) {
    let pins = [...inputs];
    const statusOn =
      (config.updateChatStatus ?? definition.defaultConfig?.updateChatStatus) ===
        true ||
      config.updateChatStatus === "true";
    if (!statusOn) {
      pins = pins.filter((pin) => pin.id !== "chatRunId");
    }
    if (definition.type === "llm:agent" || definition.type === "llm:coding_agent") {
      const mode = config.chatMode ?? definition.defaultConfig?.chatMode;
      // Graphs saved with the earlier boolean toggle migrate through its value;
      // an explicit one-message default must not mask it.
      const legacy = config.pullChatHistory;
      const historyMode =
        mode === "history" || legacy === true || legacy === "true";
      if (!historyMode) {
        pins = pins.filter((pin) => pin.id !== "chatId");
      }
    }
    return pins;
  }
  if (definition.type !== "data:build_object" || config.fields === undefined) {
    return [...inputs];
  }
  const fields = objectFieldsFromValue(
    config.fields ?? definition.defaultConfig?.fields,
  );
  return fields.map((field) => ({
    id: field.id,
    label: field.label,
    kind: "data",
    direction: "input",
    dataType: field.dataType,
    type: typeSpecFromDataType(field.dataType),
    color: dataPinColor(field.dataType),
    maxConnections: 1,
  }));
}

export function resolveConfigDrivenOutputs(
  definition: NodeDefinition | undefined,
  config: Record<string, unknown>,
): NodePort[] {
  if (!definition) return [];
  const outputs = definitionPorts(definition.outputs);
  if (definition.type === "action:javascript") {
    return resolveJavaScriptOutputs({ ...definition, outputs }, config);
  }
  if (definition.type === "discord:app_command") {
    // one output pin per command option, mirroring the Go resolver twin
    return [...outputs, ...commandOptionPorts(config.command ?? definition.defaultConfig?.command)];
  }
  if (
    definition.type === "data:base64_encode" ||
    definition.type === "data:base64_decode" ||
    definition.type === "action:file_read"
  ) {
    const representation = textBytesRepresentation(config.outputType ?? definition.defaultConfig?.outputType);
    return outputs.map((pin) =>
      pin.id === "result" ? textBytesPin(pin, representation) : pin,
    );
  }
  if (definition.type === "flow:switch") {
    const usingLegacyOptions = config.switch === undefined && config.options !== undefined;
    const configured = switchConfigFromValue(
      usingLegacyOptions ? undefined : config.switch ?? definition.defaultConfig?.switch,
      config.options,
    );
    return [
      ...configured.cases.map((item) => ({
        id: item.id,
        label: item.label,
        kind: "exec" as const,
        direction: "output" as const,
        color: "#fafafa",
        maxConnections: 1,
      })),
      ...outputs,
    ];
  }
  if (definition.type === "llm:choice") {
    const options = routeOptionsFromValue(
      config.options ?? definition.defaultConfig?.options,
    );
    return [
      ...options.map((item) => ({
        id: item.id,
        label: item.label,
        kind: "exec" as const,
        direction: "output" as const,
        color: "#fafafa",
        maxConnections: 1,
      })),
      ...outputs,
    ];
  }
  if (definition.type === "data:constant") {
    const target = config.type ?? definition.defaultConfig?.type;
    const dataType: DataType =
      target === "text" || target === "number" || target === "boolean"
        ? target
        : "any";
    return outputs.map((pin) =>
      pin.id === "value"
        ? { ...pin, dataType, type: typeSpecFromDataType(dataType), color: dataPinColor(dataType) }
        : pin,
    );
  }
  if (definition.type === "data:cast") {
    const target = config.target ?? definition.defaultConfig?.target;
    const dataType = castTargetDataType(target);
    return outputs.map((pin) =>
      pin.id === "value"
        ? { ...pin, dataType, type: castTypeSpec(dataType), color: dataPinColor(dataType) }
        : pin,
    );
  }
  if (definition.type === "data:type_assert") {
    const type = typeSpecFromValue(config.typeSpec ?? definition.defaultConfig?.typeSpec);
    const dataType = dataTypeForTypeSpec(type);
    return outputs.map((pin) =>
      pin.id === "value" ? { ...pin, dataType, type, color: dataPinColor(dataType) } : pin,
    );
  }
  if (definition.type === "data:html_extract") {
    const extractions = htmlExtractionsFromValue(
      config.extractions ?? definition.defaultConfig?.extractions,
    );
    return extractions.map((extraction) => {
      const dataType: DataType = extraction.returnAll ? "list" : "text";
      const type: TypeSpec = extraction.returnAll
        ? { kind: "list", element: { kind: "string" } }
        : { kind: "string" };
      return {
        id: extraction.id,
        label: extraction.label,
        kind: "data",
        direction: "output",
        dataType,
        type,
        color: dataPinColor(dataType),
        maxConnections: 1,
      };
    });
  }
  if (definition.type === "action:form") {
    const layout = formLayoutFromValue(config.form ?? definition.defaultConfig?.form);
    const dynamicPins = layout.items
      .filter((it: FormItemValue) => it.kind !== "text")
      .map((it: FormItemValue) => {
        const dataType: DataType = it.kind === "input" && it.inputType === "number" ? "number" : "text";
        return {
          id: it.id,
          label: it.label,
          kind: "data" as const,
          direction: "output" as const,
          dataType,
          type: typeSpecFromDataType(dataType),
          color: dataPinColor(dataType),
          maxConnections: 1,
        };
      });
    return [...dynamicPins, ...outputs];
  }
  if (
    definition.type !== "data:get_field" &&
    definition.type !== "data:break_object"
  ) {
    return [...outputs];
  }
  const hasLegacyPath =
    definition.type === "data:get_field" &&
    config.outputs === undefined &&
    typeof config.path === "string";
  const configuredOutputs = fieldOutputsFromValue(
    hasLegacyPath
      ? undefined
      : config.outputs ?? definition.defaultConfig?.outputs,
    config,
  );
  return configuredOutputs.map((output) => ({
    id: output.id,
    label: output.label,
    kind: "data",
    direction: "output",
    dataType: output.dataType,
    type: typeSpecFromDataType(output.dataType),
    color: dataPinColor(output.dataType),
    maxConnections: 1,
  }));
}


/* ------------------------------------------------------------------ */
/* send-node image/file source pin gating (Go resolver twins)          */
/* ------------------------------------------------------------------ */

/** One send node's mapping of source pin IDs onto their source mode. */
interface SourcePinSpec {
  url: string;
  file: string;
  base64: string;
  bytes: string;
  name: string;
}

const imageSourceSpec: SourcePinSpec = {
  url: "fileUrl",
  file: "filePath",
  base64: "fileBase64",
  bytes: "fileData",
  name: "fileName",
};

const photoSourceSpec: SourcePinSpec = {
  url: "photoUrl",
  file: "photoPath",
  base64: "photoBase64",
  bytes: "photoData",
  name: "photoName",
};

const documentSourceSpec: SourcePinSpec = {
  url: "documentUrl",
  file: "documentPath",
  base64: "documentBase64",
  bytes: "documentData",
  name: "fileName",
};

/** Normalises a source selector value; unknown values read as Auto (""). */
function sendSourceMode(value: unknown): string {
  return value === "url" || value === "file" || value === "base64" || value === "bytes" ? value : "";
}

/** Keeps only the pins the selected source uses; Auto keeps everything so
 * graphs saved before the selector keep their wired connections. */
function filterSourcePins(
  inputs: NodePort[],
  spec: SourcePinSpec,
  configured: unknown,
): NodePort[] {
  const mode = sendSourceMode(configured);
  if (mode === "") return inputs;
  const gated = new Set([spec.url, spec.file, spec.base64, spec.bytes, spec.name]);
  return inputs.filter((pin) => {
    if (!gated.has(pin.id)) return true;
    if (pin.id === spec.name) {
      return mode === "base64" || mode === "bytes";
    }
    return (
      (pin.id === spec.url && mode === "url") ||
      (pin.id === spec.file && mode === "file") ||
      (pin.id === spec.base64 && mode === "base64") ||
      (pin.id === spec.bytes && mode === "bytes")
    );
  });
}

/** Normalises an upload source selector value; unknown values (including the
 * send nodes' url mode) read as Auto (""). Mirrors uploadSourceMode in Go. */
function uploadSourceMode(value: unknown): string {
  return value === "file" || value === "bytes" || value === "base64" ? value : "";
}

/** Upload File pin gating: the Source dropdown keeps only the pins its mode
 * uses (localPath / data / base64); Auto keeps everything so graphs saved
 * before the selector keep their wires. Go resolver twin. */
/** commandOptionPorts grows one typed data pin per stored command option:
 * booleans become boolean pins, integers and numbers become number pins,
 * everything else stays text — exactly what the Go resolver emits. */
function commandOptionPorts(value: unknown): NodePort[] {
  const selection = (value ?? {}) as Record<string, unknown>;
  const options = Array.isArray(selection.options) ? (selection.options as Array<Record<string, unknown>>) : [];
  return options
    .filter((option) => typeof option.name === "string" && option.name !== "")
    .map((option) => {
      const type = Number(option.type ?? 3);
      const dataType: DataType = type === 5 ? "boolean" : type === 4 || type === 10 ? "number" : "text";
      const typeSpec: TypeSpec =
        type === 5
          ? { kind: "bool" }
          : type === 4
            ? { kind: "int" }
            : type === 10
              ? { kind: "float" }
              : { kind: "string" };
      return {
        id: String(option.name),
        label: String(option.name),
        kind: "data" as const,
        direction: "output" as const,
        dataType,
        type: typeSpec,
        color: dataPinColor(dataType),
        required: Boolean(option.required),
        maxConnections: 1,
      };
    });
}

function filterUploadSourcePins(inputs: NodePort[], configured: unknown): NodePort[] {
  const mode = uploadSourceMode(configured);
  if (mode === "") return inputs;
  return inputs.filter((pin) => {
    if (pin.id === "localPath") return mode === "file";
    if (pin.id === "data") return mode === "bytes";
    if (pin.id === "base64") return mode === "base64";
    return true;
  });
}

/** Draw Image pin wire types mirrored from the Go resolver. */
function drawPinDataType(pinType: string): DataType {
  switch (pinType) {
    case "number":
      return "number";
    case "boolean":
      return "boolean";
    case "object":
      return "object";
    case "array":
      return "list";
    default:
      return "text";
  }
}

function drawPinTypeSpec(pinType: string): TypeSpec {
  switch (pinType) {
    case "number":
      return { kind: "float" };
    case "boolean":
      return { kind: "bool" };
    case "object":
      return { kind: "map", key: { kind: "string" }, value: { kind: "any" } };
    case "array":
      return { kind: "list", element: { kind: "any" } };
    default:
      return { kind: "string" };
  }
}
export function formLayoutFromValue(value: unknown): FormLayoutValue {
  if (!value || typeof value !== "object") {
    return { items: [{ id: "field_1", kind: "input", label: "Input", col: 0, row: 0, span: 4, rowSpan: 1, inputType: "text" }] };
  }
  const obj = value as Record<string, unknown>;
  const itemsRaw = obj.items;
  if (!Array.isArray(itemsRaw)) {
    return { items: [{ id: "field_1", kind: "input", label: "Input", col: 0, row: 0, span: 4, rowSpan: 1, inputType: "text" }] };
  }
  const items = itemsRaw.map((raw, index) => {
    const entry = (raw ?? {}) as Record<string, unknown>;
    return {
      id: String(entry.id ?? `field_${index + 1}`),
      kind: (entry.kind === "text" || entry.kind === "input" || entry.kind === "dropdown" ? entry.kind : "input") as FormItemValue["kind"],
      label: String(entry.label ?? ""),
      col: Number(entry.col ?? 0),
      row: Number(entry.row ?? 0),
      span: Math.min(4, Math.max(1, Number(entry.span ?? 1))),
      rowSpan: Math.max(1, Number(entry.rowSpan ?? 1)),
      inputType: entry.inputType === "number" ? "number" : "text",
      placeholder: entry.placeholder ? String(entry.placeholder) : undefined,
      options: Array.isArray(entry.options)
        ? entry.options.map((opt: Record<string, unknown>) => ({ value: String(opt.value ?? ""), label: opt.label ? String(opt.label) : undefined }))
        : undefined,
    } as FormItemValue;
  });
  return { items };
}
