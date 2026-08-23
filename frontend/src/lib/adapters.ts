import type {
  ConfigField,
  CustomFunction,
  DataType,
  FlowDefinition,
  FlowEdge,
  FlowNode,
  FunctionPin,
  FunctionSummary as BackendFunctionSummary,
  NodeDefinition,
  NodePort,
} from "./types";
import type { Edge, FieldDef, GraphNode, LibraryCategory, LibraryItem, LogEntry, NodeGroup, PinDataType, Port, PortKind } from "@/types";
import { resolveConfigDrivenInputs, resolveConfigDrivenOutputs } from "./blueprint-dynamic-pins";
import { localizeNodeDefinitions } from "./node-catalog";
import i18n from "@/i18n";
import { formatDateTime } from "./format";

/* ------------------------------------------------------------------ */
/* data-type mapping                                                   */
/* ------------------------------------------------------------------ */

/** Backend PinKind → canvas port kind. */
export function mapPortKind(kind?: string): PortKind {
  if (kind === "exec" || kind === "tool") return kind;
  return "data";
}

/** Backend DataType → canvas pin type (list becomes array). */
export function mapDataType(dataType?: DataType | string): PinDataType {
  switch (dataType) {
    case "text": return "text";
    case "number": return "number";
    case "boolean": return "boolean";
    case "list": return "array";
    case "map": return "map";
    case "object": return "object";
    default: return "any";
  }
}

/** Canvas pin type → backend DataType (array becomes list). */
export function unmapDataType(dataType?: PinDataType | string): DataType {
  switch (dataType) {
    case "text": return "text";
    case "number": return "number";
    case "boolean": return "boolean";
    case "array": return "list";
    case "map":
    case "object": return "object";
    default: return "any";
  }
}

function specToArrayOf(type?: import("./types").TypeSpec): PinDataType | undefined {
  if (!type || type.kind !== "list") return undefined;
  return mapSpecToPin(type.element);
}

/** Backend TypeSpec → canvas pin type. */
export function mapSpecToPin(type?: import("./types").TypeSpec): PinDataType {
  switch (type?.kind) {
    case "string": return "text";
    case "int":
    case "float": return "number";
    case "bool": return "boolean";
    case "list": return "array";
    case "map":
    case "record": return "object";
    default: return "any";
  }
}

/* ------------------------------------------------------------------ */
/* port mapping                                                        */
/* ------------------------------------------------------------------ */

export function portFromNodePort(pin: NodePort): Port {
  const kind = mapPortKind(pin.kind);
  const dataType = kind === "exec" || kind === "tool" ? kind : mapDataType(pin.dataType);
  const port: Port = {
    id: pin.id,
    label: pin.label,
    kind,
    dataType,
    spec: pin.type,
    maxConnections: pin.maxConnections,
  };
  if (pin.type?.kind === "list") port.arrayOf = specToArrayOf(pin.type);
  if ((pin.type?.kind === "record" || pin.dataType === "object") && pin.type?.fields?.length) {
    port.objectFields = pin.type.fields.map((f) => ({
      key: f.name,
      type: mapSpecToPin(f.type),
    }));
  }
  return port;
}

/** Boundary pins of a function become editable data ports. */
export function portsFromFunctionPins(pins: FunctionPin[]): Port[] {
  return pins
    .filter((p) => p.dataType !== undefined)
    .map((p) => ({
      id: p.id,
      label: p.name || p.id,
      description: p.description,
      required: p.required,
      kind: "data" as const,
      dataType: mapDataType(p.dataType),
      spec: p.type,
      arrayOf: p.type?.kind === "list" ? mapSpecToPin(p.type.element) : undefined,
      objectFields:
        p.type?.fields?.length && p.dataType === "object"
          ? p.type.fields.map((f) => ({ key: f.name, type: mapSpecToPin(f.type) }))
          : undefined,
    }));
}

export function functionPinsFromPorts(ports: Port[], existing: FunctionPin[]): FunctionPin[] {
  return ports.map((p) => {
    const prior = existing.find((e) => e.id === p.id);
    return {
      id: p.id,
      name: p.label.trim() || p.id,
      description: p.description ?? prior?.description,
      dataType: unmapDataType(p.dataType ?? "any"),
      type: p.spec ?? prior?.type,
      required: p.required ?? prior?.required,
    };
  });
}

/* ------------------------------------------------------------------ */
/* config field mapping                                                */
/* ------------------------------------------------------------------ */

const COMPLEX_FIELD_KINDS = new Set([
  "switch-cases",
  "field-outputs",
  "object-fields",
  "html-extractions",
  "route-options",
  "http-headers",
  "json-schema",
  "form-builder",
  "sql-parameters",
]);

/** Maps a backend ConfigField onto a canvas inspector field definition. */
export function fieldDefFromConfig(field: ConfigField): FieldDef {
  const base = {
    key: field.name,
    label: field.label,
    required: field.required,
    placeholder: field.placeholder,
    kind: field.kind,
    visibleWhen: field.visibleWhen,
  };
  if (field.secret) return { ...base, type: "select", dynamic: "secrets" } as FieldDef;
  switch (field.kind) {
    case "boolean":
    case "http-user-agent-toggle":
      return { ...base, type: "toggle" } as FieldDef;
    case "number":
      return { ...base, type: "number" } as FieldDef;
    case "select":
    case "wire-representation":
    case "chat-mode":
      return { ...base, type: "select", options: field.options } as FieldDef;
    case "javascript-editor":
      return { ...base, type: "code-js" } as FieldDef;
    case "sql-editor":
      return { ...base, type: "code-sql" } as FieldDef;
    case "database-select":
      return { ...base, type: "select", dynamic: "databases" } as FieldDef;
    case "twitch-identity":
      return { ...base, type: "select", dynamic: "twitch-identity" } as FieldDef;
    case "textarea":
    case "tags":
      return { ...base, type: "textarea" } as FieldDef;
    case "json":
    case "type-spec":
    case "sql-editor-json":
      return { ...base, type: "json" } as FieldDef;
    default:
      if (COMPLEX_FIELD_KINDS.has(field.kind)) return { ...base, type: "json" } as FieldDef;
      return { ...base, type: "text" } as FieldDef;
  }
}

/** Fields filtered by their `visibleWhen` predicate against current config. */
export function visibleFields(fields: ConfigField[], values: Record<string, unknown>): ConfigField[] {
  return fields.filter((f) => {
    if (!f.visibleWhen) return true;
    if (f.visibleWhen === "chatMode") return values.chatMode === "history";
    return Boolean(values[f.visibleWhen] ?? false);
  });
}

/* ------------------------------------------------------------------ */
/* graph hydration / dehydration                                       */
/* ------------------------------------------------------------------ */

export interface DefinitionIndex {
  [type: string]: NodeDefinition;
}

export function indexDefinitions(definitions: readonly NodeDefinition[]): DefinitionIndex {
  const index: DefinitionIndex = {};
  for (const d of definitions) index[d.type] = d;
  return index;
}

const BACKEND_RESOLVED_TYPES = new Set([
  "twitch:event",
  "data:get_global_variable",
  "flow:set_global_variable",
  "action:sql",
]);

function nodeTitle(definition: NodeDefinition | undefined, type: string): string {
  return definition?.label ?? type;
}

/** Builds the canvas model for one saved flow node. */
export function hydrateNode(
  raw: FlowNode,
  definitions: DefinitionIndex,
): GraphNode {
  const type = String(raw.data?.type ?? raw.type ?? "");
  const config = (raw.data?.config ?? {}) as Record<string, unknown>;
  const definition = definitions[type];
  const inputs = definition ? resolveConfigDrivenInputs(definition, config) : [];
  const outputs = definition ? resolveConfigDrivenOutputs(definition, config) : [];
  const fields = definition ? visibleFields(definition.fields, { ...configWithDefaults(definition), ...config }).map(fieldDefFromConfig) : [];
  return {
    id: raw.id,
    type,
    title: nodeTitle(definition, type),
    icon: definition?.icon ?? "Boxes",
    group: definition?.category ?? "",
    summary: definition?.description ?? "",
    x: raw.position?.x ?? 0,
    y: raw.position?.y ?? 0,
    status: "idle",
    inputs: inputs.map(portFromNodePort),
    outputs: outputs.map(portFromNodePort),
    fields,
    values: config,
    outputSchema: outputs
      .flatMap((o) => o.fields ?? [])
      .slice(0, 6)
      .map((f) => ({ key: f.path, type: mapDataType(f.dataType ?? "any") })),
  };
}

/** Merges definition defaults under the saved config so visibleWhen works pre-edit. */
export function configWithDefaults(definition: NodeDefinition): Record<string, unknown> {
  return { ...(definition.defaultConfig ?? {}) };
}

const GROUP_COLORS = new Set(["slate", "violet", "emerald", "amber", "sky", "rose"]);

export function hydrateGraph(
  definition: FlowDefinition,
  definitions: DefinitionIndex,
): { nodes: GraphNode[]; edges: Edge[]; viewport: { x: number; y: number; z: number }; groups: NodeGroup[]; comments: import("./types").EditorCommentData[] } {
  const nodes = (definition.nodes ?? []).map((n) => hydrateNode(n, definitions));
  const edges: Edge[] = (definition.edges ?? []).map((e) => {
    const kind = mapPortKind((e as FlowEdge).kind);
    const source = nodes.find((n) => n.id === e.source);
    const port = source?.outputs.find((p) => p.id === (e.sourceHandle ?? ""));
    return {
      id: e.id,
      from: { node: e.source, port: e.sourceHandle ?? "" },
      to: { node: e.target, port: e.targetHandle ?? "" },
      kind,
      dataType: port?.dataType ?? (kind === "exec" || kind === "tool" ? kind : "any"),
      waypoints: (e as FlowEdge).waypoints,
    };
  });
  const vp = definition.viewport ?? { x: 0, y: 0, zoom: 1 };
  const groups: NodeGroup[] = ((definition as FlowDefinition).groups ?? [])
    .filter((g) => g && typeof g.id === "string")
    .map((g) => ({
      id: g.id,
      title: g.title,
      x: g.x,
      y: g.y,
      w: g.w,
      h: g.h,
      color: (GROUP_COLORS.has(g.color) ? g.color : "slate") as NodeGroup["color"],
    }));
  return { nodes, edges, viewport: { x: vp.x, y: vp.y, z: vp.zoom ?? 1 }, groups, comments: (definition as FlowDefinition).comments ?? [] };
}

/** Serialises the canvas model back into the persisted FlowDefinition parts. */
export function dehydrate(
  nodes: GraphNode[],
  edges: Edge[],
  viewport: { x: number; y: number; z: number },
  groups?: NodeGroup[],
  comments?: import("./types").EditorCommentData[],
): import("./types").FlowDefinition {
  return {
    schemaVersion: 3,
    nodes: nodes.map((n) => ({
      id: n.id,
      type: n.type,
      position: { x: n.x, y: n.y },
      data: { config: n.values },
    })),
    edges: edges.map((e) => ({
      id: e.id,
      source: e.from.node,
      target: e.to.node,
      sourceHandle: e.from.port,
      targetHandle: e.to.port,
      kind: e.kind,
      ...(e.waypoints && e.waypoints.length > 0 ? { waypoints: e.waypoints } : {}),
    })) as FlowEdge[],
    viewport: { x: viewport.x, y: viewport.y, zoom: viewport.z },
    ...(groups && groups.length > 0
      ? { groups: groups.map((g) => ({ id: g.id, title: g.title, x: g.x, y: g.y, w: g.w, h: g.h, color: g.color })) }
      : {}),
    ...(comments && comments.length > 0 ? { comments } : {}),
  };
}

/** Re-derives pins and metadata of one node after a config change. */
export function refreshNode(node: GraphNode, definitions: DefinitionIndex): GraphNode {
  const definition = definitions[node.type];
  if (!definition) return node;
  const inputs = resolveConfigDrivenInputs(definition, node.values);
  const outputs = resolveConfigDrivenOutputs(definition, node.values);
  const fields = visibleFields(definition.fields, { ...configWithDefaults(definition), ...node.values }).map(fieldDefFromConfig);
  return {
    ...node,
    title: nodeTitle(definition, node.type),
    icon: definition.icon,
    group: definition.category,
    summary: definition.description,
    inputs: inputs.map(portFromNodePort),
    outputs: outputs.map(portFromNodePort),
    fields,
    outputSchema: outputs
      .flatMap((o) => o.fields ?? [])
      .slice(0, 6)
      .map((f) => ({ key: f.path, type: mapDataType(f.dataType ?? "any") })),
  };
}

export function isBackendResolvedType(type: string): boolean {
  return BACKEND_RESOLVED_TYPES.has(type);
}

/* ------------------------------------------------------------------ */
/* library                                                             */
/* ------------------------------------------------------------------ */

export function buildLibrary(
  definitions: readonly NodeDefinition[],
  functions: readonly BackendFunctionSummary[],
): LibraryCategory[] {
  const categories = new Map<string, LibraryItem[]>();
  for (const def of definitions) {
    if (
      def.type.startsWith("trigger:") ||
      def.type === "function:entry" ||
      def.type === "function:return" ||
      def.type === "function:input" ||
      def.type === "function:output" ||
      def.type === "data:reroute" ||
      def.type === "flow:reroute"
    ) {
      continue;
    }
    const list = categories.get(def.category) ?? [];
    list.push({ name: def.label, desc: def.description, icon: def.icon, type: def.type });
    categories.set(def.category, list);
  }
  const result: LibraryCategory[] = [];
  const fnItems: LibraryItem[] = functions.map((fn) => ({
    name: fn.name,
    desc: fn.description || i18n.t("functions.noDescription"),
    icon: fn.icon || "SquareFunction",
    functionId: fn.id,
  }));
  if (fnItems.length > 0) {
    result.push({ name: i18n.t("library.functionsGroup"), count: fnItems.length, items: fnItems });
  }
  for (const [name, items] of [...categories.entries()].sort((a, b) => a[0].localeCompare(b[0]))) {
    result.push({ name, count: items.length, items });
  }
  return result;
}

export function localizeDefinitions(definitions: readonly NodeDefinition[]): NodeDefinition[] {
  return localizeNodeDefinitions(definitions, i18n.resolvedLanguage ?? i18n.language ?? "en");
}

/* ------------------------------------------------------------------ */
/* functions                                                           */
/* ------------------------------------------------------------------ */

type UiFunctionKind = "pure" | "impure" | "tool";

export function fnKindFromBackend(fn: Pick<BackendFunctionSummary, "kind" | "mode">): UiFunctionKind {
  if (fn.kind === "tool") return "tool";
  return fn.mode === "pure" ? "pure" : "impure";
}

export function fnKindToBackend(kind: UiFunctionKind): { kind: CustomFunction["kind"]; mode: CustomFunction["mode"] } {
  if (kind === "tool") return { kind: "tool", mode: "impure" };
  if (kind === "pure") return { kind: "function", mode: "pure" };
  return { kind: "function", mode: "impure" };
}

export interface UiFunctionSummary {
  id: string;
  name: string;
  desc: string;
  kind: UiFunctionKind;
  updated: string;
  category: string;
  icon: string;
  publishedRevision: number;
  /** boundary pins are only known after getFunction; summaries omit them */
  pinsLoaded: boolean;
  inputs: Port[];
  outputs: Port[];
}

export function fnSummaryFromBackend(fn: BackendFunctionSummary): UiFunctionSummary {
  return {
    id: fn.id,
    name: fn.name,
    desc: fn.description,
    kind: fnKindFromBackend(fn),
    updated: formatDateTime(fn.updatedAt),
    category: fn.category,
    icon: fn.icon || "SquareFunction",
    publishedRevision: fn.publishedRevision,
    pinsLoaded: false,
    inputs: [],
    outputs: [],
  };
}

export function fnSummaryFromDetail(fn: CustomFunction): UiFunctionSummary {
  const base = fnSummaryFromBackend(fn);
  return { ...base, pinsLoaded: true, inputs: portsFromFunctionPins(fn.inputs), outputs: portsFromFunctionPins(fn.outputs) };
}

/* ------------------------------------------------------------------ */
/* executions                                                          */
/* ------------------------------------------------------------------ */

const RUN_STATUS_MAP: Record<string, LogEntry["status"]> = {
  pending: "pending",
  running: "running",
  completed: "completed",
  failed: "failed",
  skipped: "skipped",
  cancelled: "cancelled",
};

export function logStatus(status: string): LogEntry["status"] {
  return RUN_STATUS_MAP[status] ?? "pending";
}

export function nodeRunToLog(
  run: import("./types").NodeRun,
  nodes: readonly GraphNode[],
): LogEntry {
  const node = nodes.find((n) => n.id === run.nodeId);
  const started = Date.parse(run.startedAt);
  const finished = run.finishedAt ? Date.parse(run.finishedAt) : Number.NaN;
  const ms = Number.isFinite(started) && Number.isFinite(finished) ? Math.max(0, finished - started) : 0;
  return {
    id: `${run.nodeId}:${run.startedAt}`,
    nodeId: run.nodeId,
    node: node?.title ?? run.nodeType ?? run.nodeId,
    type: run.nodeType ?? node?.type ?? "",
    status: logStatus(run.status),
    ms,
    time: formatDateTime(run.startedAt),
    startedAt: run.startedAt,
    finishedAt: run.finishedAt,
    error: run.error,
    input: run.input,
    output: run.output,
  };
}


export function applyRunStatus(status: string): GraphNode["status"] {
  switch (status) {
    case "running": return "running";
    case "completed": return "done";
    case "failed": return "error";
    case "skipped": return "idle";
    case "cancelled": return "idle";
    default: return "queued";
  }
}

/* ------------------------------------------------------------------ */
/* pipelines                                                           */
/* ------------------------------------------------------------------ */

export interface UiPipeline {
  id: string;
  name: string;
  desc: string;
  icon: string;
  status: "published" | "draft" | "legacy";
  version: string;
  triggers: number;
  updated: string;
  migrationIssue?: string;
  running?: boolean;
}

export function pipelineFromBackend(p: Omit<import("./types").PipelineSummary, "triggerCount"> & { triggerCount?: number }): UiPipeline {
  return {
    id: p.id,
    name: p.name,
    desc: p.description,
    icon: p.icon || "Cable",
    status: p.status === "active" ? "published" : p.status === "legacy" ? "legacy" : "draft",
    version: p.publishedRevision > 0 ? `v${p.publishedRevision}` : i18n.t("pipelines.draft"),
    triggers: p.triggerCount ?? 0,
    updated: formatDateTime(p.updatedAt),
    migrationIssue: p.migrationIssue,
  };
}







