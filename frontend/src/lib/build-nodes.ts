import { dataPinColor } from "@/lib/node-pins";
import { typeSpecFromDataType } from "@/lib/type-spec";
import type { DataType, NodePort } from "@/lib/types";

/**
 * Frontend twin of the Go Build Array / Build Map contract
 * (internal/nodes/data/config.go + buildarray/buildmap).
 *
 * Both nodes own one node-level collection data type — the element type for
 * Build Array and the value type for Build Map — that every pin, constant,
 * and the output share, so the collection stays homogeneous like a []T or
 * map[string]V. "any" is the explicit mixed-type escape hatch.
 *
 * Build Array persists items as [{id, label, value}] and Build Map as
 * [{id, label, key, value}]. Every row becomes one input pin under the
 * reserved "item_"/"entry_" + row-id namespace, so wires, the validator,
 * and the engine agree on stable edge handles. Row ids are minted once when
 * a row is added and never re-derived from content.
 */

export interface BuildItemRow {
  id: string;
  label: string;
  value: string;
}

export interface BuildMapEntryRow {
  id: string;
  key: string;
  label: string;
  value: string;
}

export const BUILD_ROW_TYPES = ["any", "text", "number", "boolean", "object", "list", "bytes"] as const;

export const MAX_BUILD_ROWS = 32;

export const DEFAULT_COLLECTION_TYPE = "any";

interface RawRow {
  id?: unknown;
  label?: unknown;
  key?: unknown;
  value?: unknown;
}

function asRow(raw: unknown): RawRow | null {
  if (raw === null || typeof raw !== "object" || Array.isArray(raw)) return null;
  return raw as RawRow;
}

function textOf(source: RawRow, key: keyof RawRow): string {
  const value = source[key];
  if (value === null || value === undefined) return "";
  return String(value).trim();
}

function idOf(source: RawRow, index: number): string {
  const id = textOf(source, "id");
  return id === "" ? `row_${index + 1}` : id;
}

/**
 * Parses the node-level collection type (element/value type). The Go
 * resolver rejects unsupported values at validation time; this display-side
 * parse just falls back to "any" so the editor keeps rendering.
 */
export function parseCollectionType(raw: unknown): string {
  const text = typeof raw === "string" ? raw.trim() : raw === null || raw === undefined ? "" : String(raw).trim();
  return (BUILD_ROW_TYPES as readonly string[]).includes(text) ? text : DEFAULT_COLLECTION_TYPE;
}

/** Parses the persisted item list; malformed rows are dropped. */
export function parseBuildItems(raw: unknown): BuildItemRow[] {
  if (!Array.isArray(raw)) return [];
  const rows: BuildItemRow[] = [];
  for (const entry of raw) {
    const source = asRow(entry);
    if (!source) continue;
    rows.push({
      id: idOf(source, rows.length),
      label: textOf(source, "label"),
      value: textOf(source, "value"),
    });
  }
  return rows;
}

/**
 * Parses the persisted entry list; rows with blank keys are dropped as
 * mid-edit editor state (the Go parser applies the same tolerance).
 */
export function parseMapEntries(raw: unknown): BuildMapEntryRow[] {
  if (!Array.isArray(raw)) return [];
  const rows: BuildMapEntryRow[] = [];
  for (const entry of raw) {
    const source = asRow(entry);
    if (!source) continue;
    const key = textOf(source, "key");
    if (key === "") continue;
    rows.push({
      id: idOf(source, rows.length),
      key,
      label: textOf(source, "label"),
      value: textOf(source, "value"),
    });
  }
  return rows;
}

/** Emits the persisted item payload; ids are kept verbatim. */
export function buildItemsPayload(rows: readonly BuildItemRow[]): unknown[] {
  return rows.map((row, index) => ({
    id: row.id.trim() || `row_${index + 1}`,
    label: row.label.trim(),
    value: row.value,
  }));
}

/** Emits the persisted entry payload; blank-key rows are dropped as noise. */
export function buildMapEntriesPayload(rows: readonly BuildMapEntryRow[]): unknown[] {
  return rows
    .filter((row) => row.key.trim() !== "")
    .map((row, index) => ({
      id: row.id.trim() || `row_${index + 1}`,
      label: row.label.trim(),
      key: row.key.trim(),
      value: row.value,
    }));
}

/** Next collision-free generated row id (same scheme as nextMappingID). */
export function nextBuildRowID(rows: ReadonlyArray<{ id: string }>): string {
  const used = new Set(rows.map((row) => row.id));
  for (let index = 1; ; index += 1) {
    const id = `field_${index}`;
    if (!used.has(id)) return id;
  }
}

export function itemPinID(rowID: string): string {
  return `item_${rowID}`;
}

export function entryPinID(rowID: string): string {
  return `entry_${rowID}`;
}

function rowPorts(
  rows: ReadonlyArray<{ id: string; label: string; value: string }>,
  dataType: string,
  pinID: (id: string) => string,
): NodePort[] {
  const type = dataType as DataType;
  return rows.map((row) => ({
    id: pinID(row.id),
    label: row.label.trim() || row.id,
    kind: "data" as const,
    direction: "input" as const,
    dataType: type,
    type: typeSpecFromDataType(type),
    color: dataPinColor(type),
    maxConnections: 1,
    required: row.value.trim() === "",
    default: row.value.trim() !== "" ? row.value : undefined,
  }));
}

/** Renders items as data input pins (the Go resolver twin). */
export function buildItemPorts(rows: readonly BuildItemRow[], dataType: string): NodePort[] {
  return rowPorts(rows, dataType, itemPinID);
}

/** Renders entries as data input pins (the Go resolver twin). */
export function buildEntryPorts(rows: readonly BuildMapEntryRow[], dataType: string): NodePort[] {
  return rowPorts(rows, dataType, entryPinID);
}

/** Symbolic validation codes for constants; the UI maps them to copy. */
export type LiteralIssue = "number" | "boolean" | "json" | "unsupported";

/** Mirrors the Go literal coercion so the editor flags bad constants live. */
export function literalIssue(dataType: string, value: string): LiteralIssue | null {
  const constant = value.trim();
  if (constant === "") return null;
  switch (dataType) {
    case "number":
      return Number.isFinite(Number(constant)) ? null : "number";
    case "boolean":
      return constant.toLowerCase() === "true" || constant.toLowerCase() === "false" ? null : "boolean";
    case "object": {
      try {
        return JSON.parse(constant) !== null && typeof JSON.parse(constant) === "object" && !Array.isArray(JSON.parse(constant))
          ? null
          : "json";
      } catch {
        return "json";
      }
    }
    case "list": {
      try {
        return Array.isArray(JSON.parse(constant)) ? null : "json";
      } catch {
        return "json";
      }
    }
    case "bytes":
      return "unsupported";
    default:
      return null;
  }
}

/** Coerces a constant to its declared type for the live output preview. */
export function previewValue(dataType: string, value: string): unknown {
  const constant = value.trim();
  if (constant === "") return undefined;
  switch (dataType) {
    case "number": {
      const parsed = Number(constant);
      return Number.isFinite(parsed) ? parsed : undefined;
    }
    case "boolean":
      return constant.toLowerCase() === "true" ? true : constant.toLowerCase() === "false" ? false : undefined;
    case "object":
    case "list":
      try {
        return JSON.parse(constant);
      } catch {
        return undefined;
      }
    default:
      return constant;
  }
}
