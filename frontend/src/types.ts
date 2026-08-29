/* ---- data-type system for pins ---- */

export type PinDataType =
  | "exec"
  | "tool"
  | "text"
  | "number"
  | "boolean"
  | "array"
  | "map"
  | "object"
  | "bytes"
  | "any";

export interface ObjectField {
  key: string;
  type: PinDataType | string;
}

export type PortKind = "exec" | "data" | "tool";

export interface Port {
  id: string;
  label: string;
  kind: PortKind;
  dataType?: PinDataType;
  /** full recursive contract when the backend provides one (list<map<…>> etc.) */
  spec?: import("./lib/types").TypeSpec;
  /** for array pins, inner element type */
  arrayOf?: PinDataType | string;
  /** for object pins, known structure */
  objectFields?: ObjectField[];
  /** maximum number of incoming edges this input accepts */
  maxConnections?: number;
  /** tool-contract extras carried on boundary pins */
  description?: string;
  required?: boolean;
}

export type NodeStatus = "idle" | "queued" | "running" | "done" | "error";

export type FieldType =
  | "text"
  | "textarea"
  | "number"
  | "select"
  | "toggle"
  | "json"
  | "code-js"
  | "code-sql";

export interface FieldOption {
  value: string;
  label: string;
}

export interface FieldDef {
  key: string;
  label: string;
  type: FieldType;
  required?: boolean;
  options?: FieldOption[];
  placeholder?: string;
  hint?: string;
  /** options are resolved asynchronously (secrets, databases, identities) */
  dynamic?: "secrets" | "databases" | "kv-databases" | "storages" | "twitch-identity" | "discord-identity" | "telegram-identity" | "pipelines";
  /** original backend ConfigField.kind, kept for round-tripping complex values */
  kind?: string;
  visibleWhen?: string;
}

export interface GraphNode {
  id: string;
  type: string;
  title: string;
  icon: string;
  group: string;
  summary: string;
  x: number;
  y: number;
  status: NodeStatus;
  inputs: Port[];
  outputs: Port[];
  fields: FieldDef[];
  values: Record<string, unknown>;
  outputSchema?: { key: string; type: string }[];
  locked?: boolean;
  error?: string;
  /** node run timing from the latest execution */
  lastRun?: { status: NodeStatus; ms?: number; error?: string };
}

export type FunctionKind = "pure" | "impure" | "tool";

export type GroupColor = "slate" | "violet" | "emerald" | "amber" | "sky" | "rose";

/** UI-only grouping frame. Lives in editor state, never persisted. */
export interface NodeGroup {
  id: string;
  title: string;
  x: number;
  y: number;
  w: number;
  h: number;
  color: GroupColor;
}

/** UI-only sticky note on the canvas. Persisted like groups, ignored by the engine. */
export interface EditorComment {
  id: string;
  text: string;
  x: number;
  y: number;
  w: number;
  h: number;
  color: string;
}

export interface ToolParameter {
  name: string;
  type: PinDataType;
  required: boolean;
  desc: string;
}

export interface FunctionSummary {
  id: string;
  name: string;
  desc: string;
  kind: FunctionKind;
  updated: string;
  inputs: Port[];
  outputs: Port[];
  toolParameters?: ToolParameter[];
}

export interface Edge {
  id: string;
  from: { node: string; port: string };
  to: { node: string; port: string };
  /** "tool" wires exist between an Agent's Tools pin and folded functions */
  kind: PortKind;
  dataType?: PinDataType;
  /** UI-only: waypoint knots collapse into this list on save (backend FlowEdge.waypoints) */
  waypoints?: { x: number; y: number }[];
}

export interface LibraryItem {
  name: string;
  desc: string;
  icon: string;
  /** node definition type for catalog nodes */
  type?: string;
  /** function id for folded function nodes */
  functionId?: string;
}

export interface LibraryCategory {
  name: string;
  count: number;
  items: LibraryItem[];
}

export interface LogEntry {
  id: string;
  node: string;
  type: string;
  status: "completed" | "running" | "failed" | "skipped" | "cancelled" | "pending";
  ms: number;
  time: string;
  error?: string;
  input?: unknown;
  output?: unknown;
  startedAt?: string;
  finishedAt?: string;
  /** stable node id for cross-referencing timeline ↔ entries */
  nodeId?: string;
}

