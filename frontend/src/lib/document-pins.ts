import { dataPinColor } from "@/lib/node-pins";
import type { NodePort } from "@/lib/types";

/**
 * Frontend twin of the Go dynpins contract (internal/nodes/documents/dynpins).
 *
 * Document nodes persist value bindings — placeholder values, table columns,
 * worksheet cells — as row lists; the canvas renders each row as an ordinary
 * data pin under the reserved "pin_" + row-id namespace so wires, the
 * validator, and the engine agree on stable edge handles.
 */

export interface PinBindingRow {
  id: string;
  name: string;
  label: string;
  value: string;
}

interface RawRow {
  id?: unknown;
  name?: unknown;
  label?: unknown;
  value?: unknown;
}

/** Parses the persisted pin-binding row list; malformed rows are dropped. */
export function parsePinBindings(raw: unknown): PinBindingRow[] {
  if (!Array.isArray(raw)) return [];
  const rows: PinBindingRow[] = [];
  for (const item of raw) {
    if (item === null || typeof item !== "object" || Array.isArray(item)) continue;
    const source = item as RawRow;
    const name = source.name === null || source.name === undefined ? "" : String(source.name).trim();
    if (name === "") continue; // blank rows are mid-edit editor state
    let id = source.id === null || source.id === undefined ? "" : String(source.id).trim();
    if (id === "") id = `row_${rows.length + 1}`;
    const label = source.label === null || source.label === undefined ? "" : String(source.label).trim();
    const value = source.value === null || source.value === undefined ? "" : String(source.value);
    rows.push({ id, name, label: label === "" ? name : label, value });
  }
  return rows;
}

/** Emits the persisted payload; blank rows are dropped as noise. */
export function buildPinBindingsPayload(rows: PinBindingRow[]): unknown[] {
  return rows
    .map((row) => ({
      id: row.id.trim(),
      name: row.name.trim(),
      label: row.label.trim(),
      value: row.value,
    }))
    .filter((row) => row.name !== "");
}

/** Next collision-free generated row id (same scheme as nextMappingID). */
export function nextPinBindingID(rows: readonly PinBindingRow[]): string {
  const used = new Set(rows.map((row) => row.id));
  for (let index = rows.length + 1; ; index += 1) {
    const id = `field_${index}`;
    if (!used.has(id)) return id;
  }
}

/** Renders rows as data input pins (the Go InputPins twin). */
export function pinBindingInputPorts(rows: readonly PinBindingRow[]): NodePort[] {
  return rows.map((row) => ({
    id: `pin_${row.id}`,
    label: row.label || row.name,
    kind: "data" as const,
    direction: "input" as const,
    dataType: "any",
    type: { kind: "any" },
    color: dataPinColor("any"),
    maxConnections: 1,
    default: row.value !== "" ? row.value : undefined,
  }));
}

/** Renders rows as data output pins (the Go OutputPins twin). */
export function pinBindingOutputPorts(rows: readonly PinBindingRow[]): NodePort[] {
  return rows.map((row) => ({
    id: `pin_${row.id}`,
    label: row.label || row.name,
    kind: "data" as const,
    direction: "output" as const,
    dataType: "any",
    type: { kind: "any" },
    color: dataPinColor("any"),
    maxConnections: 1,
  }));
}
