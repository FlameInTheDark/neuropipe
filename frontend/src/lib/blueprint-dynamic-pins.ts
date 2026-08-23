import { dataPinColor } from "@/lib/node-pins";
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
    case "map":
    case "record": return "object";
    default: return "any";
  }
}

type TextBytesRepresentation = "text" | "bytes";

function textBytesRepresentation(value: unknown): TextBytesRepresentation {
        return value === "text" ? "text" : "bytes";
}

function textBytesPin(pin: NodePort, representation: TextBytesRepresentation): NodePort {
        const dataType: DataType = representation === "text" ? "text" : "any";
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
    const dataType: DataType = target === "text" || target === "number" || target === "boolean" ? target : "any";
    return outputs.map((pin) =>
      pin.id === "value" ? { ...pin, dataType, type: typeSpecFromDataType(dataType), color: dataPinColor(dataType) } : pin,
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
