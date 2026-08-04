import { dataPinColor } from "@/lib/node-pins";
import type { DataType, NodeDefinition, NodePort } from "@/lib/types";

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
    cases: items.flatMap((item, index) => {
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

export function nextMappingID<T extends { id: string }>(mappings: readonly T[]) {
  const used = new Set(mappings.map((mapping) => mapping.id));
  for (let index = mappings.length + 1; ; index += 1) {
    const id = `field_${index}`;
    if (!used.has(id)) return id;
  }
}

export function resolveConfigDrivenInputs(
  definition: NodeDefinition | undefined,
  config: Record<string, unknown>,
): NodePort[] {
  if (!definition) return [];
  if (definition.type !== "data:build_object" || config.fields === undefined) {
    return [...definition.inputs];
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
    color: dataPinColor(field.dataType),
    maxConnections: 1,
  }));
}

export function resolveConfigDrivenOutputs(
  definition: NodeDefinition | undefined,
  config: Record<string, unknown>,
): NodePort[] {
  if (!definition) return [];
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
      ...definition.outputs,
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
      ...definition.outputs,
    ];
  }
  if (
    definition.type !== "data:get_field" &&
    definition.type !== "data:break_object"
  ) {
    return [...definition.outputs];
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
    color: dataPinColor(output.dataType),
    maxConnections: 1,
  }));
}
