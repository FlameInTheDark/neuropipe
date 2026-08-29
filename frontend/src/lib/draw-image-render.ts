/**
 * Draw Image canvas renderer — the HTML5 Canvas twin of the Go gg renderer.
 * Mirrors internal/nodes/local/drawimage/render.go: identical layer
 * compositing, transforms, star geometry, object-fit math, greedy wrapping,
 * and anchor semantics (baseline = y + ascent − ay·(ascent+descent)).
 */
import {
  clampRadius,
  DrawElement,
  DrawImageDoc,
  evaluateCondition,
  imageFitRect,
  interpolate,
  starVertices,
  wrapLines,
} from "./draw-image";

/* ------------------------------------------------------------------ */
/* fonts                                                               */
/* ------------------------------------------------------------------ */

const FONT_ASSETS = {
  inter: {
    family: "NPDraw Inter",
    normal: "/fonts/InterVariable.ttf",
    italic: "/fonts/InterVariable-Italic.ttf",
  },
  "jetbrains-mono": {
    family: "NPDraw Mono",
    normal: "/fonts/JetBrainsMono-Variable.ttf",
    italic: "/fonts/JetBrainsMono-VariableItalic.ttf",
  },
} as const;

export function drawFontCss(element: { fontFamily: string; weight: number; italic: boolean; fontSize: number }): string {
  const asset = element.fontFamily === "jetbrains-mono" ? FONT_ASSETS["jetbrains-mono"] : FONT_ASSETS.inter;
  const fallback = element.fontFamily === "jetbrains-mono" ? `"JetBrains Mono", ui-monospace, monospace` : `Inter, system-ui, sans-serif`;
  return `${element.italic ? "italic " : ""}${element.weight} ${element.fontSize}px "${asset.family}", ${fallback}`;
}

let fontsPromise: Promise<void> | null = null;

/** Loads the bundled TTFs (same bytes the Go renderer embeds) once. */
export function ensureDrawFonts(): Promise<void> {
  if (!fontsPromise) {
    const faces: Promise<void>[] = [];
    for (const asset of Object.values(FONT_ASSETS)) {
      for (const [style, url] of [
        ["normal", asset.normal],
        ["italic", asset.italic],
      ] as const) {
        const face = new FontFace(asset.family, `url(${url})`, { weight: "100 900", style });
        faces.push(
          face
            .load()
            .then((loaded) => {
              document.fonts.add(loaded);
            })
            .catch(() => {
              // dev-in-browser fallback: system fonts render approximately
            }),
        );
      }
    }
    fontsPromise = Promise.all(faces).then(() => undefined);
  }
  return fontsPromise;
}

/* ------------------------------------------------------------------ */
/* colors                                                              */
/* ------------------------------------------------------------------ */

export function hexToRgba(hex: string): { r: number; g: number; b: number; a: number } {
  let value = hex.startsWith("#") ? hex.slice(1) : hex;
  if (value.length === 3 || value.length === 4) {
    value = value
      .split("")
      .map((ch) => ch + ch)
      .join("");
  }
  const r = parseInt(value.slice(0, 2), 16) / 255;
  const g = parseInt(value.slice(2, 4), 16) / 255;
  const b = parseInt(value.slice(4, 6), 16) / 255;
  const a = value.length >= 8 ? parseInt(value.slice(6, 8), 16) / 255 : 1;
  return { r, g, b, a: Number.isFinite(a) ? a : 1 };
}

function cssColor(hex: string, opacity: number): string {
  const { r, g, b, a } = hexToRgba(hex);
  const alpha = Math.max(0, Math.min(1, a * opacity));
  return `rgba(${Math.round(r * 255)}, ${Math.round(g * 255)}, ${Math.round(b * 255)}, ${alpha})`;
}

/* ------------------------------------------------------------------ */
/* image cache                                                         */
/* ------------------------------------------------------------------ */

export type ImageResolver = (kind: "url" | "path" | "pin", value: string) => Promise<string>;

const imageCache = new Map<string, HTMLImageElement>();
const imageErrors = new Map<string, string>();
const pendingLoads = new Map<string, Promise<void>>();

export function cachedImage(key: string): HTMLImageElement | undefined {
  return imageCache.get(key);
}

export function imageError(key: string): string | undefined {
  return imageErrors.get(key);
}

export function clearDrawImageCache(): void {
  imageCache.clear();
  imageErrors.clear();
  pendingLoads.clear();
}

function loadImage(dataUrl: string, key: string): Promise<void> {
  return new Promise((resolve) => {
    const img = new Image();
    img.onload = () => {
      imageCache.set(key, img);
      imageErrors.delete(key);
      resolve();
    };
    img.onerror = () => {
      imageErrors.set(key, "failed to decode");
      resolve();
    };
    img.src = dataUrl;
  });
}

/** Preloads every image element source in the document through the resolver
 *  (the backend binding — no CORS). Skips sources already cached. */
export async function preloadDocImages(doc: DrawImageDoc, values: Record<string, unknown>, resolve: ImageResolver): Promise<void> {
  const jobs: Promise<void>[] = [];
  const seen = new Set<string>();
  for (const element of doc.elements) {
    if (element.type !== "image" || !element.visible) continue;
    const sources: { kind: "url" | "path" | "pin"; value: string }[] = [{ kind: element.source.kind, value: element.source.value }];
    if (element.repeat) {
      // resolved pin sources vary per item; preload the array's entries
      const items = values[element.repeat.pin];
      if (Array.isArray(items)) {
        for (const item of items) {
          if (typeof item === "string" && item !== "") sources.push({ kind: "pin", value: item });
        }
      }
    }
    for (const source of sources) {
      let key = `${source.kind}\x00${source.value}`;
      let kind = source.kind;
      let value = source.value;
      if (kind === "pin") {
        const raw = values[source.value];
        if (typeof raw !== "string" || raw.trim() === "") continue;
        value = raw.trim();
        key = `raw\x00${value}`;
        kind = value.startsWith("http://") || value.startsWith("https://") ? "url" : "path";
      }
      if (value === "" || seen.has(key) || imageCache.has(key)) continue;
      seen.add(key);
      if (!pendingLoads.has(key)) {
        const job = resolve(kind, value)
          .then((dataUrl) => loadImage(dataUrl, key))
          .catch((error) => {
            imageErrors.set(key, String(error));
          })
          .finally(() => {
            pendingLoads.delete(key);
          });
        pendingLoads.set(key, job);
        jobs.push(job);
      } else {
        jobs.push(pendingLoads.get(key)!);
      }
    }
  }
  await Promise.all(jobs);
}

function imageKeyFor(element: DrawElement, values: Record<string, unknown>): string {
  if (element.source.kind === "pin") {
    const raw = values[element.source.value];
    if (typeof raw === "string" && raw.trim() !== "") return `raw\x00${raw.trim()}`;
    return `pin\x00${element.source.value}`;
  }
  return `${element.source.kind}\x00${element.source.value}`;
}

/* ------------------------------------------------------------------ */
/* renderer                                                            */
/* ------------------------------------------------------------------ */

export interface RenderReport {
  warnings: { element: string; message: string }[];
}

export interface RenderOptions {
  /** Called for missing images instead of the default skip+warn. */
  onError?: (element: DrawElement, message: string) => void;
}

/** Renders the document onto a prepared 2D context. The context is expected
 *  to be sized doc.width × doc.height (or pre-scaled via setTransform). */
export function renderDrawImageDoc(
  ctx: CanvasRenderingContext2D,
  doc: DrawImageDoc,
  values: Record<string, unknown>,
  options: RenderOptions = {},
): RenderReport {
  const report: RenderReport = { warnings: [] };
  ctx.save();
  ctx.clearRect(0, 0, doc.width, doc.height);
  if (doc.background !== "transparent") {
    ctx.fillStyle = doc.background;
    ctx.fillRect(0, 0, doc.width, doc.height);
  }
  for (const layer of doc.layers) {
    if (!layer.visible) continue;
    const elements = doc.elements.filter((element) => element.layerId === layer.id);
    if (layer.opacity >= 1) {
      for (const element of elements) {
        drawElement(ctx, element, values, layer.opacity, report, options);
      }
      continue;
    }
    // translucent layers composite as a group, mirroring gg PushLayer
    const off = document.createElement("canvas");
    off.width = doc.width;
    off.height = doc.height;
    const offCtx = off.getContext("2d");
    if (!offCtx) continue;
    for (const element of elements) {
      drawElement(offCtx, element, values, 1, report, options);
    }
    ctx.save();
    ctx.globalAlpha = layer.opacity;
    ctx.drawImage(off, 0, 0);
    ctx.restore();
  }
  ctx.restore();
  return report;
}

function drawElement(
  ctx: CanvasRenderingContext2D,
  element: DrawElement,
  values: Record<string, unknown>,
  layerOpacity: number,
  report: RenderReport,
  options: RenderOptions,
): void {
  if (!element.visible) return;
  const opacity = element.opacity * layerOpacity;
  if (element.repeat) {
    drawRepeated(ctx, element, values, opacity, report, options);
    return;
  }
  if (!evaluateCondition(element.visibility, values)) return;
  drawOnce(ctx, element, values, opacity, report, options);
}

function drawRepeated(
  ctx: CanvasRenderingContext2D,
  element: DrawElement,
  values: Record<string, unknown>,
  opacity: number,
  report: RenderReport,
  options: RenderOptions,
): void {
  const items = values[element.repeat!.pin];
  if (!Array.isArray(items)) return;
  let count = Math.min(items.length, 100);
  const limit = element.repeat!.limit;
  if (limit > 0 && limit < count) count = limit;
  for (let index = 0; index < count; index++) {
    const ctxValues: Record<string, unknown> = { ...values, item: items[index], index };
    if (!evaluateCondition(element.visibility, ctxValues)) continue;
    const clone: DrawElement = {
      ...element,
      x: element.x + index * element.repeat!.offsetX,
      y: element.y + index * element.repeat!.offsetY,
    };
    drawOnce(ctx, clone, ctxValues, opacity, report, options);
  }
}

function drawOnce(
  ctx: CanvasRenderingContext2D,
  element: DrawElement,
  values: Record<string, unknown>,
  opacity: number,
  report: RenderReport,
  options: RenderOptions,
): void {
  switch (element.type) {
    case "text":
      drawTextElement(ctx, element, values, opacity);
      return;
    case "image":
      drawImageElement(ctx, element, values, opacity, report, options);
      return;
    case "line":
      withRotation(ctx, element, () => {
        strokePath(ctx, element, opacity, () => {
          if (element.points.length === 0) return;
          ctx.beginPath();
          ctx.moveTo(element.points[0].x, element.points[0].y);
          for (const point of element.points.slice(1)) ctx.lineTo(point.x, point.y);
        });
      });
      return;
    default:
      withRotation(ctx, element, () => {
        ctx.beginPath();
        buildShapePath(ctx, element);
        ctx.fillStyle = buildPaint(ctx, element.fill, opacity);
        ctx.fill();
        if (element.stroke && element.stroke.width > 0) {
          ctx.beginPath();
          buildShapePath(ctx, element);
          applyStroke(ctx, element.stroke, opacity);
          ctx.stroke();
        }
      });
  }
}

function withRotation(ctx: CanvasRenderingContext2D, element: DrawElement, draw: () => void): void {
  if (element.rotation === 0) {
    draw();
    return;
  }
  const cx = element.x + element.w / 2;
  const cy = element.y + element.h / 2;
  ctx.save();
  ctx.translate(cx, cy);
  ctx.rotate((element.rotation * Math.PI) / 180);
  ctx.translate(-cx, -cy);
  draw();
  ctx.restore();
}

function buildShapePath(ctx: CanvasRenderingContext2D, element: DrawElement): void {
  if (element.type === "rect") {
    roundRectPath(ctx, element.x, element.y, element.w, element.h, clampRadius(element));
    return;
  }
  if (element.type === "ellipse") {
    ctx.ellipse(element.x + element.w / 2, element.y + element.h / 2, Math.abs(element.w / 2), Math.abs(element.h / 2), 0, 0, Math.PI * 2);
    return;
  }
  if (element.type === "star") {
    const vertices = starVertices(element);
    if (vertices.length === 0) return;
    ctx.moveTo(vertices[0].x, vertices[0].y);
    for (const vertex of vertices.slice(1)) ctx.lineTo(vertex.x, vertex.y);
    ctx.closePath();
  }
}

function roundRectPath(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number): void {
  const radius = Math.max(0, Math.min(r, Math.abs(w) / 2, Math.abs(h) / 2));
  if (radius <= 0) {
    ctx.rect(x, y, w, h);
    return;
  }
  ctx.moveTo(x + radius, y);
  ctx.lineTo(x + w - radius, y);
  ctx.arcTo(x + w, y, x + w, y + radius, radius);
  ctx.lineTo(x + w, y + h - radius);
  ctx.arcTo(x + w, y + h, x + w - radius, y + h, radius);
  ctx.lineTo(x + radius, y + h);
  ctx.arcTo(x, y + h, x, y + h - radius, radius);
  ctx.lineTo(x, y + radius);
  ctx.arcTo(x, y, x + radius, y, radius);
  ctx.closePath();
}

function buildPaint(ctx: CanvasRenderingContext2D, paint: DrawElement["fill"], opacity: number): string | CanvasGradient {
  if (paint.type === "linear") {
    const gradient = ctx.createLinearGradient(paint.x0, paint.y0, paint.x1, paint.y1);
    for (const stop of paint.stops) gradient.addColorStop(stop.offset, cssColor(stop.color, opacity));
    return gradient;
  }
  if (paint.type === "radial") {
    const gradient = ctx.createRadialGradient(paint.cx, paint.cy, 0, paint.cx, paint.cy, paint.r);
    for (const stop of paint.stops) gradient.addColorStop(stop.offset, cssColor(stop.color, opacity));
    return gradient;
  }
  return cssColor(paint.color, opacity);
}

function applyStroke(ctx: CanvasRenderingContext2D, stroke: NonNullable<DrawElement["stroke"]>, opacity: number): void {
  ctx.strokeStyle = cssColor(stroke.color, opacity);
  ctx.lineWidth = stroke.width;
  ctx.lineCap = stroke.cap;
  ctx.lineJoin = stroke.join;
  ctx.setLineDash(stroke.dash);
}

function strokePath(ctx: CanvasRenderingContext2D, element: DrawElement, opacity: number, buildPath: () => void): void {
  if (!element.stroke || element.stroke.width <= 0) return;
  applyStroke(ctx, element.stroke, opacity);
  buildPath();
  ctx.stroke();
  ctx.setLineDash([]);
}

/* ------------------------------------------------------------------ */
/* text                                                                */
/* ------------------------------------------------------------------ */

function drawTextElement(ctx: CanvasRenderingContext2D, element: DrawElement, values: Record<string, unknown>, opacity: number): void {
  ctx.font = drawFontCss(element);
  const content = interpolate(element.content, values);
  const wrapLimit = element.wrapWidth === -1 ? element.w : element.wrapWidth;
  const lines = wrapLines(content, wrapLimit, (line) => ctx.measureText(line).width);

  const lineAdvance = element.fontSize * element.lineHeight;
  const blockHeight = lines.length * lineAdvance;

  const anchorX = element.align === "center" ? element.x + element.w / 2 : element.align === "right" ? element.x + element.w : element.x;
  const blockTop = element.valign === "middle" ? element.y + (element.h - blockHeight) / 2 : element.valign === "bottom" ? element.y + element.h - blockHeight : element.y;

  // anchor metrics mirroring gg: baseline = y + ascent − ay·(ascent+descent)
  const metrics = ctx.measureText("Mg");
  const ascent = metrics.fontBoundingBoxAscent || element.fontSize * 0.8;
  const descent = metrics.fontBoundingBoxDescent || element.fontSize * 0.2;
  const half = (ascent + descent) / 2;
  const baselineOffset = ascent - half;

  withRotation(ctx, element, () => {
    ctx.fillStyle = cssColor(element.color, opacity);
    ctx.textBaseline = "alphabetic";
    ctx.textAlign = element.align === "center" ? "center" : element.align === "right" ? "right" : "left";
    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      if (line === "") continue;
      const y = blockTop + i * lineAdvance + lineAdvance / 2 + baselineOffset;
      ctx.fillText(line, anchorX, y);
    }
  });
}

/* ------------------------------------------------------------------ */
/* images                                                              */
/* ------------------------------------------------------------------ */

function drawImageElement(
  ctx: CanvasRenderingContext2D,
  element: DrawElement,
  values: Record<string, unknown>,
  opacity: number,
  report: RenderReport,
  options: RenderOptions,
): void {
  const key = imageKeyFor(element, values);
  const img = imageCache.get(key);
  if (!img) {
    const message = imageErrors.get(key) ?? "image not loaded yet";
    report.warnings.push({ element: element.name, message });
    options.onError?.(element, message);
    return;
  }
  const fit = imageFitRect(img.naturalWidth, img.naturalHeight, { x: element.x, y: element.y, w: element.w, h: element.h }, element.fit);
  withRotation(ctx, element, () => {
    ctx.save();
    ctx.globalAlpha = opacity;
    if (element.radius > 0) {
      ctx.beginPath();
      roundRectPath(ctx, element.x, element.y, element.w, element.h, clampRadius(element));
      ctx.clip();
    }
    ctx.drawImage(img, fit.sx, fit.sy, fit.sw, fit.sh, fit.dx, fit.dy, fit.dw, fit.dh);
    ctx.restore();
  });
}

/* ------------------------------------------------------------------ */
/* thumbnail helper                                                    */
/* ------------------------------------------------------------------ */

/** Renders a small preview of the document (inspector thumbnail). */
export function renderThumbnail(
  canvas: HTMLCanvasElement,
  doc: DrawImageDoc,
  values: Record<string, unknown>,
  maxWidth: number,
  maxHeight: number,
): void {
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  const scale = Math.min(maxWidth / doc.width, maxHeight / doc.height, 1);
  const width = Math.max(1, Math.round(doc.width * scale));
  const height = Math.max(1, Math.round(doc.height * scale));
  canvas.width = width;
  canvas.height = height;
  ctx.setTransform(width / doc.width, 0, 0, height / doc.height, 0, 0);
  renderDrawImageDoc(ctx, doc, values);
  ctx.setTransform(1, 0, 0, 1, 0, 0);
}
