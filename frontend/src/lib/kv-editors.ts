import type { TypeSpec } from "@/lib/types";

/**
 * Shared models for the KV node's visual field editors. Rows are the
 * user-facing view; payloads are what the Go side unmarshals from node
 * config. Every parser accepts both the array/object shape the new editors
 * emit and the legacy JSON/textarea strings older pipelines still carry, so
 * saved flows keep working unchanged.
 */

/* ---------------- string lists (kv-string-list) ---------------- */

/** Parses an array of values, or a legacy newline-separated textarea string. */
export function parseKvStringList(raw: unknown): string[] {
  let list: unknown = raw;
  if (typeof raw === "string") {
    try {
      // The JSON editor persisted `["a", "b"]` as a string before this editor.
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) list = parsed;
    } catch {
      /* plain textarea content */
    }
  }
  if (typeof list === "string") {
    return list.split("\n").map((line) => line.trim()).filter((line) => line !== "");
  }
  if (!Array.isArray(list)) return [];
  return list.map((item) => {
    if (item === null || item === undefined) return "";
    if (typeof item === "object") {
      try {
        return JSON.stringify(item);
      } catch {
        return String(item);
      }
    }
    return String(item);
  });
}

/** Emits a plain string array; blank rows are dropped as noise. */
export function buildKvStringListPayload(rows: string[]): string[] {
  return rows.map((row) => row.trim()).filter((row) => row !== "");
}

/* ---------------- hash fields (kv-hash-fields) ---------------- */

export interface KvHashFieldRow {
  field: string;
  value: string;
}

/** Parses a {field: value} object or a legacy JSON string. */
export function parseKvHashFields(raw: unknown): KvHashFieldRow[] {
  let object: unknown = raw;
  if (typeof raw === "string") {
    try {
      object = JSON.parse(raw);
    } catch {
      return [];
    }
  }
  if (!object || typeof object !== "object" || Array.isArray(object)) return [];
  return Object.entries(object as Record<string, unknown>).map(([field, value]) => ({
    field,
    value: value === null || value === undefined ? "" : String(value),
  }));
}

/** Emits a field→value object; duplicate fields keep the last row (map semantics). */
export function buildKvHashFieldsPayload(rows: KvHashFieldRow[]): Record<string, string> {
  const result: Record<string, string> = {};
  for (const row of rows) {
    const field = row.field.trim();
    if (field === "") continue;
    result[field] = row.value;
  }
  return result;
}

/* ---------------- scored entries (kv-scored-entries) ---------------- */

export interface KvScoredEntryRow {
  member: string;
  score: string;
}

/** Parses an array of {member, score} objects or a legacy JSON string. */
export function parseKvScoredEntries(raw: unknown): KvScoredEntryRow[] {
  let list: unknown = raw;
  if (typeof raw === "string") {
    try {
      list = JSON.parse(raw);
    } catch {
      return [];
    }
  }
  if (!Array.isArray(list)) return [];
  return list
    .filter((item): item is Record<string, unknown> => typeof item === "object" && item !== null)
    .map((item) => ({
      member: item.member === null || item.member === undefined ? "" : String(item.member),
      score: item.score === null || item.score === undefined ? "" : String(item.score),
    }));
}

/** Emits {member, score} objects; scores become numbers when parseable. */
export function buildKvScoredEntriesPayload(rows: KvScoredEntryRow[]): unknown[] {
  const result: { member: string; score: number | string }[] = [];
  for (const row of rows) {
    const member = row.member.trim();
    if (member === "") continue;
    const trimmedScore = row.score.trim();
    const numeric = Number(trimmedScore);
    result.push({
      member,
      score: trimmedScore !== "" && Number.isFinite(numeric) ? numeric : trimmedScore,
    });
  }
  return result;
}

/* ---------------- command arguments (kv-arguments) ---------------- */

export type KvArgKind = "any" | "string" | "int" | "float" | "bool" | "list" | "map";

export interface KvArgumentRow {
  name: string;
  label: string;
  kind: KvArgKind;
  required: boolean;
}

const KIND_SPECS: Record<KvArgKind, TypeSpec> = {
  any: { kind: "any" },
  string: { kind: "string" },
  int: { kind: "int" },
  float: { kind: "float" },
  bool: { kind: "bool" },
  list: { kind: "list", element: { kind: "any" } },
  map: { kind: "map", key: { kind: "string" }, value: { kind: "any" } },
};

export function kvArgSpec(kind: KvArgKind): TypeSpec {
  return KIND_SPECS[kind] ?? KIND_SPECS.any;
}

function kindFromSpec(spec: TypeSpec | undefined): KvArgKind {
  switch (spec?.kind) {
    case "string":
      return "string";
    case "int":
      return "int";
    case "float":
      return "float";
    case "bool":
      return "bool";
    case "list":
      return "list";
    case "map":
    case "record":
      return "map";
    default:
      return "any";
  }
}

/** Parses the persisted KVArgument contract (array or JSON string). */
export function parseKvArguments(raw: unknown): KvArgumentRow[] {
  let list: unknown = raw;
  if (typeof raw === "string") {
    try {
      list = JSON.parse(raw);
    } catch {
      return [];
    }
  }
  if (!Array.isArray(list)) return [];
  return list
    .filter((p): p is Record<string, unknown> => typeof p === "object" && p !== null)
    .map((p) => ({
      name: typeof p.name === "string" ? p.name : typeof p.id === "string" ? p.id : "",
      label: typeof p.label === "string" ? p.label : "",
      kind: kindFromSpec(p.type as TypeSpec | undefined),
      required: p.required === true,
    }));
}

/** Builds the persisted payload; derives identifier-safe ids from names. */
export function buildKvArgumentPayload(rows: KvArgumentRow[]): unknown[] {
  return rows.map((row, index) => {
    const id = /^[A-Za-z_][A-Za-z0-9_]*$/.test(row.name) ? row.name : `arg_${index + 1}`;
    return {
      id,
      name: id,
      label: row.label.trim() || id,
      type: kvArgSpec(row.kind),
      required: row.required,
    };
  });
}
