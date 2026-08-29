import { dataPinColor } from "@/lib/node-pins";
import type { DataType, NodeDefinition, NodePort, TypeSpec } from "@/lib/types";

export type JavaScriptCapability = "file-read" | "file-write" | "network";

export interface JavaScriptPinContract {
  id: string;
  label: string;
  type: TypeSpec;
  required: boolean;
}

export interface JavaScriptNodeConfig {
  code: string;
  inputs: JavaScriptPinContract[];
  outputs: JavaScriptPinContract[];
  capabilities: JavaScriptCapability[];
}

const identifier = /^[A-Za-z_$][A-Za-z0-9_$]*$/;
const reserved = new Set([
  "arguments", "await", "break", "case", "catch", "class", "const", "continue",
  "debugger", "default", "delete", "do", "else", "enum", "eval", "export", "extends",
  "false", "finally", "for", "function", "if", "implements", "import", "in", "instanceof",
  "interface", "let", "new", "null", "package", "private", "protected", "public", "return",
  "super", "switch", "static", "this", "throw", "true", "try", "typeof", "undefined", "var",
  "void", "while", "with", "yield", "inputs", "np", "code",
]);

const capabilities: readonly JavaScriptCapability[] = ["file-read", "file-write", "network"];

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function typeSpec(value: unknown): TypeSpec {
  if (!record(value) || typeof value.kind !== "string") return { kind: "any" };
  switch (value.kind) {
    case "any":
    case "bool":
    case "string":
    case "int":
    case "float":
    case "bytes":
      return { kind: value.kind };
    case "list":
      return { kind: "list", element: typeSpec(value.element) };
    case "map":
      return { kind: "map", key: { kind: "string" }, value: typeSpec(value.value) };
    case "record":
      return {
        kind: "record",
        name: typeof value.name === "string" ? value.name : undefined,
        fields: Array.isArray(value.fields)
          ? value.fields.flatMap((field) => {
            if (!record(field)) return [];
            const name = typeof field.name === "string"
              ? field.name.trim()
              : typeof field.id === "string" ? field.id.trim() : "";
            if (!name) return [];
            return [{
              id: typeof field.id === "string" && field.id.trim() ? field.id.trim() : name,
              name,
              type: typeSpec(field.type),
              optional: field.optional === true,
            }];
          })
          : [],
      };
    default:
      return { kind: "any" };
  }
}

function contracts(value: unknown): JavaScriptPinContract[] {
  if (!Array.isArray(value)) return [];
  const used = new Set<string>();
  return value.flatMap((item) => {
    if (!record(item) || typeof item.id !== "string") return [];
    const id = item.id.trim();
    if (!id || used.has(id)) return [];
    used.add(id);
    return [{
      id,
      label: typeof item.label === "string" && item.label.trim() ? item.label.trim() : id,
      type: typeSpec(item.type),
      required: item.required === true,
    }];
  });
}

function dataTypeFor(type: TypeSpec): DataType {
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

export function defaultJavaScriptNodeConfig(
  config: Record<string, unknown> | undefined,
): JavaScriptNodeConfig {
  const allowed = Array.isArray(config?.capabilities)
    ? config.capabilities.filter((item): item is JavaScriptCapability =>
      typeof item === "string" && capabilities.includes(item as JavaScriptCapability),
    )
    : [];
  return {
    code: typeof config?.code === "string" ? config.code : "return {};",
    inputs: contracts(config?.inputs),
    outputs: contracts(config?.outputs),
    capabilities: [...new Set(allowed)],
  };
}

function ports(contracts: readonly JavaScriptPinContract[], direction: "input" | "output"): NodePort[] {
  return contracts.map((contract) => {
    const dataType = dataTypeFor(contract.type);
    return {
      id: contract.id,
      label: contract.label,
      kind: "data",
      direction,
      dataType,
      type: contract.type,
      color: dataPinColor(dataType),
      required: contract.required,
      maxConnections: direction === "input" ? 1 : undefined,
    };
  });
}

export function resolveJavaScriptInputs(
  definition: NodeDefinition,
  config: Record<string, unknown>,
): NodePort[] {
  const inputs = Array.isArray(definition.inputs) ? definition.inputs : [];
  return [...inputs, ...ports(defaultJavaScriptNodeConfig(config).inputs, "input")];
}

export function resolveJavaScriptOutputs(
  definition: NodeDefinition,
  config: Record<string, unknown>,
): NodePort[] {
  const outputs = Array.isArray(definition.outputs) ? definition.outputs : [];
  return [...outputs, ...ports(defaultJavaScriptNodeConfig(config).outputs, "output")];
}

export function isJavaScriptIdentifier(value: string) {
  return identifier.test(value) && !reserved.has(value);
}

function typeScriptType(type: TypeSpec, depth = 0): string {
  if (depth > 6) return "unknown";
  switch (type.kind) {
    case "bool": return "boolean";
    case "string": return "string";
    case "int":
    case "float": return "number";
    case "bytes": return "Uint8Array";
    case "list": return `Array<${type.element ? typeScriptType(type.element, depth + 1) : "unknown"}>`;
    case "map": return `Record<string, ${type.value ? typeScriptType(type.value, depth + 1) : "unknown"}>`;
    case "record": return `{ ${type.fields?.map((field) => `${JSON.stringify(field.name || field.id)}${field.optional ? "?" : ""}: ${typeScriptType(field.type, depth + 1)}`).join("; ") ?? "[key: string]: unknown"} }`;
    default: return "unknown";
  }
}

export function javascriptDeclarations(config: JavaScriptNodeConfig) {
  const inputLines = config.inputs.map((input) =>
    `declare const ${input.id}: ${typeScriptType(input.type)};`,
  );
  return [
    "declare const inputs: Record<string, unknown>;",
    "declare const np: {",
    "  readonly context: { nodeId: string; pipelineId?: string; executionId?: string };",
    "  uuid(): string; assert(condition: unknown, message?: string): void; fail(message?: string): never;",
    "  variables: { get(name: string): unknown; has(name: string): boolean; set(name: string, value: unknown): void; delete(name: string): void };",
    "  base64: { encodeText(value: string): string; decodeText(value: string): string; encodeBytes(value: Uint8Array): string; decodeBytes(value: string): Uint8Array };",
    "  hash: { sha256(value: string | Uint8Array): string };",
    "  getPipelines(): Array<{ id: string; name: string; description: string; status: string }>;",
    "  pipelines: { list(): Array<{ id: string; name: string; description: string; status: string }>; get(id: string): unknown };",
    "  functions: { list(): Array<{ id: string; name: string; description: string }> };",
    "  triggers: { list(): Array<{ id: string; label: string; kind: string; enabled: boolean }> };",
    "  executions: { list(limit?: number): Array<{ id: string; status: string }> };",
    "  reports: { list(limit?: number): unknown[]; get(id: string): unknown; create(report: { title: string; markdown: string; tags?: string[] }): unknown };",
    "  chat: { history(id: string, limit?: number): unknown[]; reply(runId: string, message: string): unknown; setStatus(runId: string, status: string): void };",
    "  files: { list(path: string): unknown[]; readBytes(path: string): Uint8Array; readText(path: string): string; writeBytes(path: string, data: Uint8Array): { path: string; written: boolean }; writeText(path: string, text: string): { path: string; written: boolean } };",
    "  http: { request(request: { url: string; method?: string; headers?: Record<string, string | string[]>; body?: string | Uint8Array }): { status: number; headers: Record<string, string[]>; body: string } };",
    "  notify(title: string, message: string): void;",
    "};",
    ...inputLines,
  ].join("\n");
}
