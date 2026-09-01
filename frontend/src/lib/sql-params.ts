import type { TypeSpec } from "@/lib/types";

/**
 * Shared model for the SQL node's parameter contract (config key
 * `parameters`). Rows are the user-facing view; payloads are what the Go
 * side unmarshals into domain.SQLParameter. The full recursive TypeSpec is
 * preserved so list<map<string, any>> style contracts survive round-trips.
 */

export interface SqlParamRow {
  name: string;
  label: string;
  /** ui-level shorthand rendered as a select */
  kind: "text" | "int" | "float" | "bool";
  /** full recursive contract; takes precedence over `kind` when present */
  spec?: TypeSpec;
  required: boolean;
}

/** Wire contracts derived from the UI-level kind shorthand. */
const KIND_SPECS: Record<NonNullable<SqlParamRow["kind"]>, TypeSpec> = {
  text: { kind: "string" },
  int: { kind: "int" },
  float: { kind: "float" },
  bool: { kind: "bool" },
};

/** Parses whatever sits in config.parameters (array or JSON string). */
export function parseSqlParams(raw: unknown): SqlParamRow[] {
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
    .map((p) => {
      const spec = (p as { type?: TypeSpec }).type;
      const kind = uiKind(spec?.kind);
      return {
        name: typeof p.name === "string" ? p.name : "",
        label: typeof p.label === "string" ? p.label : "",
        kind,
        spec: spec ?? KIND_SPECS[kind],
        required: p.required === true,
      };
    });
}

/** Maps a wire contract kind onto the UI-level shorthand select. */
function uiKind(kind?: string): NonNullable<SqlParamRow["kind"]> {
  switch (kind) {
    case "int": return "int";
    case "float": return "float";
    case "bool": return "bool";
    default: return "text";
  }
}

/** Builds the persisted payload; derives stable ids from names. */
export function buildSqlPayload(rows: SqlParamRow[]): unknown[] {
  return rows.map((r, i) => {
    const id = /^[A-Za-z_][A-Za-z0-9_]*$/.test(r.name) ? r.name : `param_${i + 1}`;
    const spec = r.spec ?? KIND_SPECS[r.kind];
    return { id, name: id, label: r.label.trim() || id, type: spec, required: r.required };
  });
}

/** Static input pin that carries a replacement statement over a wire. */
export const SQL_PIN_ID = "sql";
