/**
 * Draw Image document model — the TypeScript twin of the Go package
 * internal/nodes/local/drawimage. Every parse default, clamp, interpolation
 * rule, condition operator, and geometry formula is mirrored exactly so the
 * editor canvas preview matches the backend gg render.
 */

export const MAX_CANVAS_DIMENSION = 8192;
export const MAX_REPEAT_COPIES = 100;

export type DrawPinType = "text" | "number" | "boolean" | "object" | "array";

export type DrawElementType = "rect" | "ellipse" | "line" | "star" | "text" | "image";

export interface DrawPin {
  name: string;
  type: DrawPinType;
  sample: string;
  default: string;
}

export interface DrawLayer {
  id: string;
  name: string;
  visible: boolean;
  opacity: number;
  locked: boolean;
}

export interface GradientStop {
  offset: number;
  color: string;
}

export type DrawPaint =
  | { type: "solid"; color: string }
  | { type: "linear"; x0: number; y0: number; x1: number; y1: number; stops: GradientStop[] }
  | { type: "radial"; cx: number; cy: number; r: number; stops: GradientStop[] };

export interface DrawStroke {
  color: string;
  width: number;
  dash: number[];
  cap: "butt" | "round" | "square";
  join: "miter" | "round" | "bevel";
}

export interface DrawVisibility {
  mode: "always" | "condition";
  pin: string;
  op: string;
  value: string;
}

export interface DrawRepeat {
  pin: string;
  offsetX: number;
  offsetY: number;
  limit: number;
}

export interface DrawPoint {
  x: number;
  y: number;
}

export interface DrawImageSource {
  kind: "url" | "path" | "pin";
  value: string;
}

export interface DrawElement {
  id: string;
  type: DrawElementType;
  layerId: string;
  name: string;
  x: number;
  y: number;
  w: number;
  h: number;
  rotation: number;
  opacity: number;
  visible: boolean;
  visibility: DrawVisibility;
  repeat: DrawRepeat | null;
  /** rect + image */
  radius: number;
  fill: DrawPaint;
  stroke: DrawStroke | null;
  /** line */
  points: DrawPoint[];
  /** star */
  starPoints: number;
  innerRatio: number;
  /** text */
  content: string;
  fontFamily: "inter" | "jetbrains-mono";
  fontSize: number;
  weight: number;
  italic: boolean;
  color: string;
  align: "left" | "center" | "right";
  valign: "top" | "middle" | "bottom";
  lineHeight: number;
  wrapWidth: number;
  /** image */
  source: DrawImageSource;
  fit: "fill" | "contain" | "cover";
  onMissing: "skip" | "error";
}

export interface DrawImageDoc {
  version: number;
  width: number;
  height: number;
  background: string;
  pins: DrawPin[];
  layers: DrawLayer[];
  elements: DrawElement[];
}

/* ------------------------------------------------------------------ */
/* parse helpers                                                       */
/* ------------------------------------------------------------------ */

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function str(value: unknown, fallback: string): string {
  return typeof value === "string" ? value : fallback;
}

function num(value: unknown, fallback: number): number {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value.trim());
    if (Number.isFinite(parsed)) return parsed;
  }
  return fallback;
}

function int(value: unknown, fallback: number): number {
  const parsed = num(value, fallback);
  return Number.isInteger(parsed) ? parsed : fallback;
}

function bool(value: unknown, fallback: boolean): boolean {
  return typeof value === "boolean" ? value : fallback;
}

function clamp(value: number, lo: number, hi: number): number {
  return Math.min(hi, Math.max(lo, value));
}

function oneOf<T extends string>(value: unknown, fallback: T, allowed: readonly T[]): T {
  const text = typeof value === "string" ? (value as T) : fallback;
  return allowed.includes(text) ? text : fallback;
}

const HEX_RE = /^[0-9a-f]+$/;

function parseColorValue(value: unknown, fallback: string): string {
  const trimmed = typeof value === "string" ? value.trim() : "";
  if (!trimmed.startsWith("#")) return fallback;
  const hex = trimmed.slice(1).toLowerCase();
  if ([3, 4, 6, 8].includes(hex.length) && HEX_RE.test(hex)) return `#${hex}`;
  return fallback;
}

function parseBackground(value: unknown): string {
  const trimmed = typeof value === "string" ? value.trim().toLowerCase() : "";
  if (trimmed === "" || trimmed === "transparent" || trimmed === "none") return "transparent";
  const colored = parseColorValue(trimmed, "");
  return colored || "#0b0c0d";
}

const PIN_NAME_RE = /^[A-Za-z_][A-Za-z0-9_]*$/;

export const RESERVED_PIN_NAMES = new Set([
  "in",
  "out",
  "image",
  "base64",
  "result",
  "outputPath",
  "item",
  "index",
]);

export function isValidPinName(name: string): boolean {
  return PIN_NAME_RE.test(name) && !RESERVED_PIN_NAMES.has(name);
}

/* ------------------------------------------------------------------ */
/* document parsing                                                    */
/* ------------------------------------------------------------------ */

const DEFAULT_PAINT: DrawPaint = { type: "solid", color: "#141516" };

export function normalizeDrawImageDoc(value: unknown): DrawImageDoc {
  const doc: DrawImageDoc = {
    version: 1,
    width: 800,
    height: 450,
    background: "#0b0c0d",
    pins: [],
    layers: [{ id: "layer_1", name: "Layer 1", visible: true, opacity: 1, locked: false }],
    elements: [],
  };
  const root = isRecord(value) ? value : {};
  doc.version = clamp(int(root.version, 1), 1, 99);
  doc.width = clamp(int(root.width, doc.width), 1, MAX_CANVAS_DIMENSION);
  doc.height = clamp(int(root.height, doc.height), 1, MAX_CANVAS_DIMENSION);
  doc.background = parseBackground(root.background);

  if (Array.isArray(root.pins)) {
    const seen = new Set<string>();
    for (const raw of root.pins) {
      const entry = isRecord(raw) ? raw : {};
      const name = str(entry.name, "").trim();
      if (!isValidPinName(name) || seen.has(name)) continue;
      seen.add(name);
      doc.pins.push({
        name,
        type: oneOf(entry.type, "text", ["text", "number", "boolean", "object", "array"]),
        sample: str(entry.sample, ""),
        default: str(entry.default, ""),
      });
    }
  }

  const layerIds = new Set<string>();
  if (Array.isArray(root.layers) && root.layers.length > 0) {
    for (const raw of root.layers) {
      const entry = isRecord(raw) ? raw : {};
      const id = str(entry.id, "");
      if (id === "" || layerIds.has(id)) continue;
      layerIds.add(id);
      doc.layers.push({
        id,
        name: str(entry.name, id),
        visible: bool(entry.visible, true),
        opacity: clamp(num(entry.opacity, 1), 0, 1),
        locked: bool(entry.locked, false),
      });
    }
    doc.layers = doc.layers.slice(1);
  }
  if (doc.layers.length === 0) {
    doc.layers = [{ id: "layer_1", name: "Layer 1", visible: true, opacity: 1, locked: false }];
    layerIds.add("layer_1");
  }
  const firstLayerId = doc.layers[0].id;
  doc.layers.forEach((layer) => layerIds.add(layer.id));

  if (Array.isArray(root.elements)) {
    for (const raw of root.elements) {
      const element = normalizeElement(raw, layerIds, firstLayerId);
      if (element) doc.elements.push(element);
    }
  }
  return doc;
}

export function normalizeElement(
  value: unknown,
  layerIds: Set<string>,
  firstLayerId: string,
): DrawElement | null {
  const entry = isRecord(value) ? value : {};
  const type = oneOf(entry.type, "rect", ["rect", "ellipse", "line", "star", "text", "image"]);
  const id = str(entry.id, "");
  if (id === "") return null;
  const layerIdRaw = str(entry.layerId, "");
  const layerId = layerIds.has(layerIdRaw) ? layerIdRaw : firstLayerId;
  const base: DrawElement = {
    id,
    type,
    layerId,
    name: str(entry.name, defaultElementName(type)),
    x: num(entry.x, 0),
    y: num(entry.y, 0),
    w: clamp(num(entry.w, 100), -MAX_CANVAS_DIMENSION * 2, MAX_CANVAS_DIMENSION * 2),
    h: clamp(num(entry.h, 100), -MAX_CANVAS_DIMENSION * 2, MAX_CANVAS_DIMENSION * 2),
    rotation: clamp(num(entry.rotation, 0), -360, 360),
    opacity: clamp(num(entry.opacity, 1), 0, 1),
    visible: bool(entry.visible, true),
    visibility: normalizeVisibility(entry.visibility),
    repeat: normalizeRepeat(entry.repeat),
    radius: clamp(num(entry.radius, 0), 0, MAX_CANVAS_DIMENSION),
    fill: normalizePaint(entry.fill),
    stroke: normalizeStroke(entry.stroke),
    points: normalizePoints(entry.points),
    starPoints: clamp(int(entry.points, 5), 3, 24),
    innerRatio: clamp(num(entry.innerRatio, 0.5), 0.05, 0.95),
    content: str(entry.content, ""),
    fontFamily: oneOf(entry.fontFamily, "inter", ["inter", "jetbrains-mono"]),
    fontSize: clamp(num(entry.fontSize, 24), 1, 512),
    weight: clamp(int(entry.weight, 400), 100, 900),
    italic: bool(entry.italic, false),
    color: parseColorValue(entry.color, "#f7f8f8"),
    align: oneOf(entry.align, "left", ["left", "center", "right"]),
    valign: oneOf(entry.valign, "top", ["top", "middle", "bottom"]),
    lineHeight: clamp(num(entry.lineHeight, 1.2), 0.5, 3),
    // wrapWidth: -1 wraps to the element width, 0 disables wrapping.
    wrapWidth: clamp(num(entry.wrapWidth, 0), -1, MAX_CANVAS_DIMENSION),
    source: normalizeImageSource(entry.source),
    fit: oneOf(entry.fit, "cover", ["fill", "contain", "cover"]),
    onMissing: oneOf(entry.onMissing, "skip", ["skip", "error"]),
  };
  return base;
}

export function defaultElementName(type: DrawElementType): string {
  switch (type) {
    case "rect":
      return "Rectangle";
    case "ellipse":
      return "Ellipse";
    case "line":
      return "Line";
    case "star":
      return "Star";
    case "text":
      return "Text";
    case "image":
      return "Image";
  }
}

function normalizeVisibility(value: unknown): DrawVisibility {
  const entry = isRecord(value) ? value : {};
  const mode = oneOf(entry.mode, "always", ["always", "condition"]);
  if (mode === "always") return { mode: "always", pin: "", op: "", value: "" };
  const pin = str(entry.pin, "").trim();
  const op = str(entry.op, "");
  if (pin === "" || op === "") return { mode: "always", pin: "", op: "", value: "" };
  return { mode: "condition", pin, op, value: str(entry.value, "") };
}

function normalizeRepeat(value: unknown): DrawRepeat | null {
  const entry = isRecord(value) ? value : {};
  const pin = str(entry.pin, "").trim();
  if (pin === "") return null;
  return {
    pin,
    offsetX: clamp(num(entry.offsetX, 0), -MAX_CANVAS_DIMENSION * 2, MAX_CANVAS_DIMENSION * 2),
    offsetY: clamp(num(entry.offsetY, 0), -MAX_CANVAS_DIMENSION * 2, MAX_CANVAS_DIMENSION * 2),
    limit: clamp(int(entry.limit, 0), 0, MAX_REPEAT_COPIES),
  };
}

function normalizePaint(value: unknown): DrawPaint {
  const entry = isRecord(value) ? value : {};
  const type = oneOf(entry.type, "solid", ["solid", "linear", "radial"]);
  if (type === "linear") {
    const stops = normalizeStops(entry.stops);
    if (stops.length === 0) return { ...DEFAULT_PAINT };
    return {
      type,
      x0: num(entry.x0, 0),
      y0: num(entry.y0, 0),
      x1: num(entry.x1, 100),
      y1: num(entry.y1, 0),
      stops,
    };
  }
  if (type === "radial") {
    const stops = normalizeStops(entry.stops);
    if (stops.length === 0) return { ...DEFAULT_PAINT };
    return { type, cx: num(entry.cx, 50), cy: num(entry.cy, 50), r: clamp(num(entry.r, 50), 0.01, MAX_CANVAS_DIMENSION), stops };
  }
  return { type: "solid", color: parseColorValue(entry.color, "#141516") };
}

function normalizeStops(value: unknown): GradientStop[] {
  if (!Array.isArray(value)) return [];
  const stops: GradientStop[] = [];
  for (const raw of value) {
    const entry = isRecord(raw) ? raw : {};
    stops.push({ offset: clamp(num(entry.offset, 0), 0, 1), color: parseColorValue(entry.color, "#ffffff") });
  }
  return stops;
}

function normalizeStroke(value: unknown): DrawStroke | null {
  const entry = isRecord(value) ? value : {};
  const width = clamp(num(entry.width, 1), 0, 200);
  if (width <= 0) return null;
  return {
    color: parseColorValue(entry.color, "#232326"),
    width,
    dash: Array.isArray(entry.dash)
      ? entry.dash.map((d) => clamp(num(d, 0), 0, 2000)).filter((d) => d > 0)
      : [],
    cap: oneOf(entry.cap, "butt", ["butt", "round", "square"]),
    join: oneOf(entry.join, "miter", ["miter", "round", "bevel"]),
  };
}

function normalizePoints(value: unknown): DrawPoint[] {
  if (!Array.isArray(value)) return [];
  const points: DrawPoint[] = [];
  for (const raw of value) {
    const entry = isRecord(raw) ? raw : {};
    points.push({ x: num(entry.x, 0), y: num(entry.y, 0) });
  }
  return points;
}

function normalizeImageSource(value: unknown): DrawImageSource {
  const entry = isRecord(value) ? value : {};
  const kind = oneOf(entry.kind, "url", ["url", "path", "pin"]);
  return { kind, value: str(entry.value, "").trim() };
}

/* ------------------------------------------------------------------ */
/* serialization                                                       */
/* ------------------------------------------------------------------ */

/** Serializes the doc for config storage (JSON string keeps the config bag
 *  small and matches how complex field values round-trip). */
export function serializeDrawImageDoc(doc: DrawImageDoc): unknown {
  return {
    version: doc.version,
    width: doc.width,
    height: doc.height,
    background: doc.background,
    pins: doc.pins.map((pin) => ({ name: pin.name, type: pin.type, sample: pin.sample, default: pin.default })),
    layers: doc.layers.map((layer) => ({
      id: layer.id,
      name: layer.name,
      visible: layer.visible,
      opacity: round(layer.opacity),
      locked: layer.locked,
    })),
    elements: doc.elements.map((element) => {
      const base: Record<string, unknown> = {
        id: element.id,
        type: element.type,
        layerId: element.layerId,
        name: element.name,
        x: round(element.x),
        y: round(element.y),
        w: round(element.w),
        h: round(element.h),
        rotation: round(element.rotation),
        opacity: round(element.opacity),
        visible: element.visible,
        visibility: element.visibility.mode === "always"
          ? { mode: "always" }
          : { mode: "condition", pin: element.visibility.pin, op: element.visibility.op, value: element.visibility.value },
      };
      if (element.repeat) {
        base.repeat = { pin: element.repeat.pin, offsetX: round(element.repeat.offsetX), offsetY: round(element.repeat.offsetY), limit: element.repeat.limit };
      }
      if (element.type === "rect") {
        base.radius = round(element.radius);
        base.fill = serializePaint(element.fill);
        base.stroke = element.stroke ? serializeStroke(element.stroke) : null;
      } else if (element.type === "ellipse" || element.type === "star") {
        base.fill = serializePaint(element.fill);
        base.stroke = element.stroke ? serializeStroke(element.stroke) : null;
        if (element.type === "star") {
          base.points = element.starPoints;
          base.innerRatio = round(element.innerRatio);
        }
      } else if (element.type === "line") {
        base.points = element.points.map((p) => ({ x: round(p.x), y: round(p.y) }));
        base.stroke = element.stroke ? serializeStroke(element.stroke) : null;
        if (element.points.length > 0) {
          const xs = element.points.map((p) => p.x);
          const ys = element.points.map((p) => p.y);
          base.x = round(Math.min(...xs));
          base.y = round(Math.min(...ys));
          base.w = round(Math.max(...xs) - Math.min(...xs));
          base.h = round(Math.max(...ys) - Math.min(...ys));
        }
      } else if (element.type === "text") {
        base.content = element.content;
        base.fontFamily = element.fontFamily;
        base.fontSize = round(element.fontSize);
        base.weight = element.weight;
        base.italic = element.italic;
        base.color = element.color;
        base.align = element.align;
        base.valign = element.valign;
        base.lineHeight = round(element.lineHeight);
        base.wrapWidth = round(element.wrapWidth);
      } else if (element.type === "image") {
        base.source = { kind: element.source.kind, value: element.source.value };
        base.fit = element.fit;
        base.radius = round(element.radius);
        base.onMissing = element.onMissing;
      }
      return base;
    }),
  };
}

function serializePaint(paint: DrawPaint): unknown {
  if (paint.type === "solid") return { type: "solid", color: paint.color };
  if (paint.type === "linear") {
    return { type: "linear", x0: round(paint.x0), y0: round(paint.y0), x1: round(paint.x1), y1: round(paint.y1), stops: paint.stops.map(serializeStop) };
  }
  return { type: "radial", cx: round(paint.cx), cy: round(paint.cy), r: round(paint.r), stops: paint.stops.map(serializeStop) };
}

function serializeStop(stop: GradientStop): unknown {
  return { offset: round(stop.offset), color: stop.color };
}

function serializeStroke(stroke: DrawStroke): unknown {
  return {
    color: stroke.color,
    width: round(stroke.width),
    dash: stroke.dash.map(round),
    cap: stroke.cap,
    join: stroke.join,
  };
}

function round(value: number): number {
  return Math.round(value * 100) / 100;
}

/* ------------------------------------------------------------------ */
/* template interpolation                                              */
/* ------------------------------------------------------------------ */

const PLACEHOLDER_RE = /\{\{\s*([A-Za-z0-9_.]+)\s*}}/g;

export type TemplateContext = Record<string, unknown>;

export function interpolate(content: string, ctx: TemplateContext): string {
  if (!content.includes("{{")) return content;
  return content.replace(PLACEHOLDER_RE, (_match, name: string) => stringifyValue(resolvePath(ctx, name)));
}

function resolvePath(ctx: TemplateContext, path: string): unknown {
  const parts = path.split(".");
  if (parts.length === 0) return undefined;
  let current: unknown = ctx[parts[0]];
  if (current === undefined) return undefined;
  for (const part of parts.slice(1)) {
    if (current === null || current === undefined) return undefined;
    if (Array.isArray(current)) {
      const index = Number(part);
      if (!Number.isInteger(index) || index < 0 || index >= current.length) return undefined;
      current = current[index];
    } else if (isRecord(current)) {
      if (!(part in current)) return undefined;
      current = current[part];
    } else {
      return undefined;
    }
  }
  return current;
}

/** Mirrors Go StringifyValue: text as-is, shortest number, bool literal,
 *  arrays joined with ", ", objects as sorted compact JSON, null → "". */
export function stringifyValue(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  if (typeof value === "boolean") return value ? "true" : "false";
  if (typeof value === "number") return formatNumber(value);
  if (Array.isArray(value)) return value.map(stringifyValue).join(", ");
  if (isRecord(value)) return compactSortedJSON(value);
  return String(value);
}

function formatNumber(value: number): string {
  if (!Number.isFinite(value)) return "0";
  return String(value);
}

function compactSortedJSON(value: Record<string, unknown>): string {
  const keys = Object.keys(value).sort();
  const parts: string[] = [];
  for (const key of keys) {
    parts.push(`${JSON.stringify(key)}:${jsonScalar(value[key])}`);
  }
  return `{${parts.join(",")}}`;
}

function jsonScalar(value: unknown): string {
  if (isRecord(value)) return compactSortedJSON(value);
  if (Array.isArray(value)) return `[${value.map(jsonScalar).join(",")}]`;
  if (value === null || value === undefined) return "null";
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return JSON.stringify(value);
  }
  return JSON.stringify(String(value));
}

/* ------------------------------------------------------------------ */
/* visibility conditions                                               */
/* ------------------------------------------------------------------ */

export function isPseudoPin(name: string): boolean {
  return name === "item" || name === "index" || name.startsWith("item.");
}

export function evaluateCondition(
  visibility: DrawVisibility,
  values: Record<string, unknown>,
  _pinType?: DrawPinType | "",
): boolean {
  if (visibility.mode !== "condition") return true;
  const value = isPseudoPin(visibility.pin)
    ? resolvePath(values as TemplateContext, visibility.pin)
    : values[visibility.pin];
  return applyOp(visibility.op, value, visibility.value);
}

function applyOp(op: string, value: unknown, operand: string): boolean {
  switch (op) {
    case "isEmpty":
      return valueIsEmpty(value);
    case "notEmpty":
      return !valueIsEmpty(value);
    case "isTrue":
      return valueTruthy(value);
    case "isFalse":
      return !valueTruthy(value);
    case "eq":
      return stringifyValue(value) === operand;
    case "ne":
      return stringifyValue(value) !== operand;
    case "contains":
      return stringifyValue(value).includes(operand);
    case "notContains":
      return !stringifyValue(value).includes(operand);
    case "startsWith":
      return stringifyValue(value).startsWith(operand);
    case "endsWith":
      return stringifyValue(value).endsWith(operand);
    case "gt":
      return toNumber(value) > toNumber(operand);
    case "ge":
      return toNumber(value) >= toNumber(operand);
    case "lt":
      return toNumber(value) < toNumber(operand);
    case "le":
      return toNumber(value) <= toNumber(operand);
    case "arrayContains":
      return Array.isArray(value) && value.some((item) => stringifyValue(item) === operand);
    case "arrayNotContains":
      return !(Array.isArray(value) && value.some((item) => stringifyValue(item) === operand));
    case "lenEq":
      return arrayLength(value) === toNumber(operand);
    case "lenNe":
      return arrayLength(value) !== toNumber(operand);
    case "lenGt":
      return arrayLength(value) > toNumber(operand);
    case "lenGe":
      return arrayLength(value) >= toNumber(operand);
    case "lenLt":
      return arrayLength(value) < toNumber(operand);
    case "lenLe":
      return arrayLength(value) <= toNumber(operand);
    case "hasKey":
      return isRecord(value) && operand in value;
    default:
      return true;
  }
}

function valueIsEmpty(value: unknown): boolean {
  if (value === null || value === undefined) return true;
  if (typeof value === "string") return value === "";
  if (Array.isArray(value)) return value.length === 0;
  if (isRecord(value)) return Object.keys(value).length === 0;
  return stringifyValue(value) === "";
}

function valueTruthy(value: unknown): boolean {
  if (value === null || value === undefined) return false;
  if (typeof value === "boolean") return value;
  if (typeof value === "string") return value === "true" || value === "1";
  if (typeof value === "number") return value !== 0;
  return false;
}

function toNumber(value: unknown): number {
  if (typeof value === "number") return Number.isFinite(value) ? value : 0;
  if (typeof value === "boolean") return value ? 1 : 0;
  if (typeof value === "string") {
    const parsed = Number(value.trim());
    return Number.isFinite(parsed) ? parsed : 0;
  }
  if (Array.isArray(value)) return value.length;
  if (isRecord(value)) return Object.keys(value).length;
  return 0;
}

function arrayLength(value: unknown): number {
  if (Array.isArray(value)) return value.length;
  if (typeof value === "string") return value.split(",").length;
  if (isRecord(value)) return Object.keys(value).length;
  return 0;
}

/** Operators offered per pin type in the condition builder. */
export const CONDITION_OPS: Record<string, { value: string; labelKey: string }[]> = {
  boolean: [
    { value: "isTrue", labelKey: "drawImage.ops.isTrue" },
    { value: "isFalse", labelKey: "drawImage.ops.isFalse" },
    { value: "isEmpty", labelKey: "drawImage.ops.isEmpty" },
    { value: "notEmpty", labelKey: "drawImage.ops.notEmpty" },
  ],
  number: [
    { value: "eq", labelKey: "drawImage.ops.eq" },
    { value: "ne", labelKey: "drawImage.ops.ne" },
    { value: "gt", labelKey: "drawImage.ops.gt" },
    { value: "ge", labelKey: "drawImage.ops.ge" },
    { value: "lt", labelKey: "drawImage.ops.lt" },
    { value: "le", labelKey: "drawImage.ops.le" },
    { value: "isEmpty", labelKey: "drawImage.ops.isEmpty" },
    { value: "notEmpty", labelKey: "drawImage.ops.notEmpty" },
  ],
  text: [
    { value: "eq", labelKey: "drawImage.ops.eq" },
    { value: "ne", labelKey: "drawImage.ops.ne" },
    { value: "contains", labelKey: "drawImage.ops.contains" },
    { value: "notContains", labelKey: "drawImage.ops.notContains" },
    { value: "startsWith", labelKey: "drawImage.ops.startsWith" },
    { value: "endsWith", labelKey: "drawImage.ops.endsWith" },
    { value: "isEmpty", labelKey: "drawImage.ops.isEmpty" },
    { value: "notEmpty", labelKey: "drawImage.ops.notEmpty" },
  ],
  array: [
    { value: "arrayContains", labelKey: "drawImage.ops.arrayContains" },
    { value: "arrayNotContains", labelKey: "drawImage.ops.arrayNotContains" },
    { value: "lenEq", labelKey: "drawImage.ops.lenEq" },
    { value: "lenNe", labelKey: "drawImage.ops.lenNe" },
    { value: "lenGt", labelKey: "drawImage.ops.lenGt" },
    { value: "lenGe", labelKey: "drawImage.ops.lenGe" },
    { value: "lenLt", labelKey: "drawImage.ops.lenLt" },
    { value: "lenLe", labelKey: "drawImage.ops.lenLe" },
    { value: "isEmpty", labelKey: "drawImage.ops.isEmpty" },
    { value: "notEmpty", labelKey: "drawImage.ops.notEmpty" },
  ],
  object: [
    { value: "hasKey", labelKey: "drawImage.ops.hasKey" },
    { value: "isEmpty", labelKey: "drawImage.ops.isEmpty" },
    { value: "notEmpty", labelKey: "drawImage.ops.notEmpty" },
  ],
  pseudo: [
    { value: "eq", labelKey: "drawImage.ops.eq" },
    { value: "ne", labelKey: "drawImage.ops.ne" },
    { value: "gt", labelKey: "drawImage.ops.gt" },
    { value: "lt", labelKey: "drawImage.ops.lt" },
    { value: "contains", labelKey: "drawImage.ops.contains" },
    { value: "isEmpty", labelKey: "drawImage.ops.isEmpty" },
    { value: "notEmpty", labelKey: "drawImage.ops.notEmpty" },
  ],
};

/** Ops that need a comparison value input. */
export const OPS_WITHOUT_VALUE = new Set([
  "isTrue",
  "isFalse",
  "isEmpty",
  "notEmpty",
]);

/* ------------------------------------------------------------------ */
/* word wrap                                                           */
/* ------------------------------------------------------------------ */

export function wrapLines(content: string, limit: number, measure: (text: string) => number): string[] {
  const paragraphs = content.replace(/\r\n/g, "\n").split("\n");
  const lines: string[] = [];
  for (const paragraph of paragraphs) {
    if (limit <= 0) {
      lines.push(paragraph);
      continue;
    }
    const words = paragraph.split(/\s+/).filter((word) => word !== "");
    if (words.length === 0) {
      lines.push("");
      continue;
    }
    let current = "";
    for (const word of words) {
      const candidate = current === "" ? word : `${current} ${word}`;
      if (current === "" || measure(candidate) <= limit) {
        current = candidate;
        continue;
      }
      lines.push(current);
      current = word;
    }
    if (current !== "") lines.push(current);
  }
  if (lines.length === 0) lines.push("");
  return lines;
}

/* ------------------------------------------------------------------ */
/* geometry (shared with renderer)                                     */
/* ------------------------------------------------------------------ */

/** Star vertices in canvas coordinates — identical math to Go buildStarPath. */
export function starVertices(element: {
  x: number;
  y: number;
  w: number;
  h: number;
  starPoints: number;
  innerRatio: number;
}): DrawPoint[] {
  const cx = element.x + element.w / 2;
  const cy = element.y + element.h / 2;
  const outer = Math.min(Math.abs(element.w), Math.abs(element.h)) / 2;
  const inner = outer * element.innerRatio;
  const count = Math.max(3, element.starPoints);
  const vertices: DrawPoint[] = [];
  for (let i = 0; i < count * 2; i++) {
    const angle = -Math.PI / 2 + (i * Math.PI) / count;
    const radius = i % 2 === 1 ? inner : outer;
    vertices.push({ x: cx + radius * Math.cos(angle), y: cy + radius * Math.sin(angle) });
  }
  return vertices;
}

export function clampRadius(element: { w: number; h: number; radius: number }): number {
  const half = Math.min(Math.abs(element.w), Math.abs(element.h)) / 2;
  return clamp(element.radius, 0, half);
}

/** object-fit math — identical to Go drawImageElement. */
export function imageFitRect(
  srcW: number,
  srcH: number,
  box: { x: number; y: number; w: number; h: number },
  fit: "fill" | "contain" | "cover",
): { dx: number; dy: number; dw: number; dh: number; sx: number; sy: number; sw: number; sh: number } {
  if (fit === "contain") {
    const scale = Math.min(box.w / srcW, box.h / srcH);
    const dw = srcW * scale;
    const dh = srcH * scale;
    return { dx: box.x + (box.w - dw) / 2, dy: box.y + (box.h - dh) / 2, dw, dh, sx: 0, sy: 0, sw: srcW, sh: srcH };
  }
  if (fit === "cover") {
    const scale = Math.max(box.w / srcW, box.h / srcH);
    const cropW = box.w / scale;
    const cropH = box.h / scale;
    const sx = (srcW - cropW) / 2;
    const sy = (srcH - cropH) / 2;
    return { dx: box.x, dy: box.y, dw: box.w, dh: box.h, sx, sy, sw: cropW, sh: cropH };
  }
  return { dx: box.x, dy: box.y, dw: box.w, dh: box.h, sx: 0, sy: 0, sw: srcW, sh: srcH };
}

/* ------------------------------------------------------------------ */
/* editor helpers                                                      */
/* ------------------------------------------------------------------ */

export function nextId(prefix: string, used: Set<string>): string {
  for (let index = used.size + 1; ; index += 1) {
    const id = `${prefix}_${index}`;
    if (!used.has(id)) return id;
  }
}

export function createElement(
  type: DrawElementType,
  layerId: string,
  doc: DrawImageDoc,
  nameIndex: number,
  bounds?: { x: number; y: number; w: number; h: number },
): DrawElement {
  const used = new Set([...doc.layers.map((l) => l.id), ...doc.elements.map((e) => e.id)]);
  const x = bounds?.x ?? Math.round(doc.width * 0.25);
  const y = bounds?.y ?? Math.round(doc.height * 0.25);
  const w = bounds?.w ?? Math.round(doc.width * 0.25);
  const h = bounds?.h ?? Math.round(doc.height * 0.2);
  const element: DrawElement = {
    id: nextId("el", used),
    type,
    layerId,
    name: `${defaultElementName(type)} ${nameIndex}`,
    x,
    y,
    w,
    h,
    rotation: 0,
    opacity: 1,
    visible: true,
    visibility: { mode: "always", pin: "", op: "", value: "" },
    repeat: null,
    radius: type === "rect" ? 8 : 0,
    fill:
      type === "text"
        ? { type: "solid", color: "#f7f8f8" }
        : { type: "solid", color: "#4ea7fc" },
    stroke: null,
    points:
      type === "line"
        ? [
            { x, y: y + h },
            { x: x + w, y },
          ]
        : [],
    starPoints: 5,
    innerRatio: 0.5,
    content: type === "text" ? "Text" : "",
    fontFamily: "inter",
    fontSize: Math.max(12, Math.round(Math.min(doc.height * 0.08, 32))),
    weight: 400,
    italic: false,
    color: "#f7f8f8",
    align: "left",
    valign: "top",
    lineHeight: 1.2,
    wrapWidth: 0,
    source: { kind: "url", value: "" },
    fit: "cover",
    onMissing: "skip",
  };
  if (type === "text") {
    element.wrapWidth = -1; // wrap to element width
    element.valign = "top";
  }
  if (type === "line") {
    element.stroke = { color: "#f7f8f8", width: 2, dash: [], cap: "round", join: "round" };
    element.w = w;
    element.h = h;
  }
  if (type === "star") {
    element.fill = { type: "solid", color: "#f0bf00" };
  }
  if (type === "image") {
    element.fill = { type: "solid", color: "#141516" };
  }
  return element;
}

/** Parses a sample value string into a runtime value of the pin's type. */
export function sampleValue(pin: DrawPin): unknown {
  const text = pin.sample.trim();
  if (text === "") {
    if (pin.type === "array") return [];
    if (pin.type === "object") return {};
    if (pin.type === "boolean") return false;
    if (pin.type === "number") return 0;
    return "";
  }
  switch (pin.type) {
    case "number": {
      const parsed = Number(text);
      return Number.isFinite(parsed) ? parsed : 0;
    }
    case "boolean":
      return text === "true" || text === "1";
    case "object":
    case "array":
      try {
        const parsed = JSON.parse(text);
        if (pin.type === "array" && Array.isArray(parsed)) return parsed;
        if (pin.type === "object" && isRecord(parsed)) return parsed;
      } catch {
        /* fall through to text */
      }
      return text;
    default:
      return text;
  }
}

export function sampleValuesFor(doc: DrawImageDoc): Record<string, unknown> {
  const values: Record<string, unknown> = {};
  for (const pin of doc.pins) {
    values[pin.name] = sampleValue(pin);
  }
  return values;
}

/** Elements referencing a pin (visibility, repeat, image source, text). */
export function pinUsage(doc: DrawImageDoc, pinName: string): number {
  let count = 0;
  for (const element of doc.elements) {
    if (element.visibility.mode === "condition" && element.visibility.pin === pinName) count += 1;
    if (element.repeat?.pin === pinName) count += 1;
    if (element.type === "image" && element.source.kind === "pin" && element.source.value === pinName) count += 1;
    if (element.type === "text" && element.content.includes(`{{${pinName}}}`)) count += 1;
  }
  return count;
}

/** Element bbox (for line elements: points bbox). */
export function elementBounds(element: DrawElement): { x: number; y: number; w: number; h: number } {
  if (element.type === "line" && element.points.length > 0) {
    const xs = element.points.map((p) => p.x);
    const ys = element.points.map((p) => p.y);
    const minX = Math.min(...xs);
    const minY = Math.min(...ys);
    return { x: minX, y: minY, w: Math.max(...xs) - minX, h: Math.max(...ys) - minY };
  }
  return { x: element.x, y: element.y, w: element.w, h: element.h };
}
