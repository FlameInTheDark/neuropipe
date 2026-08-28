import type { PinDataType, PortKind } from "../types";

/**
 * Single source of truth for pin data-types and their visual language.
 * Previously duplicated across Inspector, CodeEditorModal and pin-colors.
 *
 * Colors are CSS variables (defined per theme in index.css) so the palette
 * flips with [data-theme]. SVG presentation attributes cannot resolve
 * var(), so canvas strokes/fills must be applied via style props.
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

function pal(type: PinDataType, name: string): PinPalette {
  const v = `--pin-${type}`;
  return {
    dot: `var(${v})`,
    bg: `var(${v})`,
    hover: `var(${v}-strong)`,
    edge: `var(${v}-deep)`,
    label: `var(${v}-strong)`,
    name,
  };
}

const PALETTES: Record<PinDataType, PinPalette> = {
  exec: pal("exec", "Exec"),
  tool: pal("tool", "Tool"),
  text: pal("text", "Text"),
  number: pal("number", "Number"),
  boolean: pal("boolean", "Boolean"),
  array: pal("array", "Array"),
  map: pal("map", "Map"),
  object: pal("object", "Object"),
  any: pal("any", "Any"),
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

const DANGER = "var(--status-danger)";

export function edgeColor(
  kind: PortKind,
  dataType?: PinDataType,
  active?: boolean,
  hover?: boolean,
): string {
  if (hover) return DANGER; // hovering an edge means "click to delete"
  const p = pinPalette(kind === "exec" || kind === "tool" ? kind : dataType);
  return active ? p.hover : p.edge;
}
