import type { PinDataType, PortKind } from "../types";

/**
 * Single source of truth for pin data-types and their visual language.
 * Previously duplicated across Inspector, CodeEditorModal and pin-colors.
 */

export interface PinPalette {
  /** border/fill of the port dot */
  dot: string;
  /** fill when the pin is connected */
  bg: string;
  /** border colour on hover */
  hover: string;
  /** stroke colour of the bezier connection */
  edge: string;
  /** text colour used in tooltips/badges */
  label: string;
  /** human readable name */
  name: string;
}

const PALETTES: Record<PinDataType, PinPalette> = {
  exec:    { dot: "#c9c9d2", bg: "#c9c9d2", hover: "#ecedf1", edge: "#7c7c88", label: "#c9c9d2", name: "Exec" },
  tool:    { dot: "#818cf8", bg: "#818cf8", hover: "#a5b4fc", edge: "#4f46e5", label: "#a5b4fc", name: "Tool" },
  text:    { dot: "#f472b6", bg: "#f472b6", hover: "#f9a8d4", edge: "#db2777", label: "#f9a8d4", name: "Text" },
  number:  { dot: "#38bdf8", bg: "#38bdf8", hover: "#7dd3fc", edge: "#0284c7", label: "#7dd3fc", name: "Number" },
  boolean: { dot: "#f87171", bg: "#f87171", hover: "#fca5a5", edge: "#dc2626", label: "#fca5a5", name: "Boolean" },
  array:   { dot: "#a78bfa", bg: "#a78bfa", hover: "#c4b5fd", edge: "#7c3aed", label: "#c4b5fd", name: "Array" },
  map:     { dot: "#fb923c", bg: "#fb923c", hover: "#fdba74", edge: "#ea580c", label: "#fdba74", name: "Map" },
  object:  { dot: "#34d399", bg: "#34d399", hover: "#6ee7b7", edge: "#059669", label: "#6ee7b7", name: "Object" },
  any:     { dot: "#94a3b8", bg: "#94a3b8", hover: "#cbd5e1", edge: "#64748b", label: "#cbd5e1", name: "Any" },
};

/** data-types a user may assign to an editable pin (exec is structural, not selectable) */
export const ASSIGNABLE_PIN_TYPES: PinDataType[] = [
  "text", "number", "boolean", "array", "map", "object", "any",
];

export function portKindFromDataType(dataType?: PinDataType): PortKind {
  if (dataType === "exec" || dataType === "tool") return dataType;
  return "data";
}

export function pinPalette(dataType?: PinDataType): PinPalette {
  return PALETTES[dataType ?? "any"];
}

/** convenience for the many places that only need the dot colour */
export function pinColor(dataType?: string): string {
  return PALETTES[(dataType ?? "any") as PinDataType]?.dot ?? PALETTES.any.dot;
}

const DANGER = "#fb7185";

export function edgeColor(
  kind: PortKind,
  dataType?: PinDataType,
  active?: boolean,
  hover?: boolean,
): string {
  if (hover) return DANGER; // hovering an edge means "click to delete"
  const pal = pinPalette(kind === "exec" || kind === "tool" ? kind : dataType);
  return active ? pal.hover : pal.edge;
}
