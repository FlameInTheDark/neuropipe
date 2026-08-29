import type { MenuItem } from "../ContextMenu";
import type { DrawElement, DrawElementType, DrawImageDoc } from "@/lib/draw-image";
import { elementTypeIcon } from "./shared";

/**
 * Context menu factories for the Draw Image canvas. Pure functions — the menu
 * structure lives next to the canvas domain, the editor shell only supplies
 * the actions. Mirrors the graph editor-menus.ts pattern.
 */

type TFunc = (key: string, vars?: Record<string, unknown>) => string;

/* ------------------------------------------------------------------ */
/* element menu                                                        */
/* ------------------------------------------------------------------ */

export interface DrawElementMenuActions {
  editProperties(id: string): void;
  duplicate(id: string): void;
  toggleVisible(id: string, visible: boolean): void;
  moveZ(id: string, direction: -1 | 1): void;
  reorder(id: string, target: "front" | "back"): void;
  rotate(id: string, degrees: number): void;
  center(id: string, axis: "h" | "v"): void;
  /** line elements only — appends a point continuing the last segment */
  addPoint(id: string): void;
  remove(id: string): void;
  unlockLayer(layerId: string): void;
  showLayer(layerId: string): void;
}

const later = (fn: () => void) => () => window.setTimeout(fn, 0);

export function buildDrawElementMenu(
  element: DrawElement,
  doc: DrawImageDoc,
  a: DrawElementMenuActions,
  t: TFunc,
): MenuItem[] {
  const layer = doc.layers.find((l) => l.id === element.layerId);
  const index = doc.elements.findIndex((e) => e.id === element.id);
  const topmost = index === doc.elements.length - 1;
  const bottom = index === 0;
  const layerStateItems: MenuItem[] = [];
  if (layer?.locked) {
    layerStateItems.push({ label: t("drawImage.unlockLayer"), icon: "Unlock", onSelect: later(() => a.unlockLayer(layer.id)) });
  }
  if (layer && !layer.visible) {
    layerStateItems.push({ label: t("drawImage.showLayer"), icon: "Eye", onSelect: later(() => a.showLayer(layer.id)) });
  }

  return [
    /* pseudo-header: what was clicked */
    { label: element.name, icon: elementTypeIcon(element.type), hint: layer?.name, disabled: true },
    ...layerStateItems,
    ...(layerStateItems.length > 0 ? [{ type: "sep" } as MenuItem] : []),
    { label: t("drawImage.ctxEditProperties"), icon: "Pencil", onSelect: later(() => a.editProperties(element.id)) },
    { label: t("drawImage.duplicateElement"), icon: "Copy", hint: "⌘D", onSelect: later(() => a.duplicate(element.id)) },
    {
      label: element.visible ? t("drawImage.hideElement") : t("drawImage.showElement"),
      icon: element.visible ? "EyeOff" : "Eye",
      onSelect: later(() => a.toggleVisible(element.id, !element.visible)),
    },
    { type: "sep" },
    { label: t("drawImage.ctxBringToFront"), icon: "ArrowUpToLine", disabled: topmost, onSelect: later(() => a.reorder(element.id, "front")) },
    { label: t("drawImage.ctxBringForward"), icon: "ChevronUp", disabled: topmost, onSelect: later(() => a.moveZ(element.id, 1)) },
    { label: t("drawImage.ctxSendBackward"), icon: "ChevronDown", disabled: bottom, onSelect: later(() => a.moveZ(element.id, -1)) },
    { label: t("drawImage.ctxSendToBack"), icon: "ArrowDownToLine", disabled: bottom, onSelect: later(() => a.reorder(element.id, "back")) },
    { type: "sep" },
    { label: t("drawImage.ctxRotateCw"), icon: "RotateCw", onSelect: later(() => a.rotate(element.id, 90)) },
    { label: t("drawImage.ctxRotateCcw"), icon: "RotateCcw", onSelect: later(() => a.rotate(element.id, -90)) },
    { label: t("drawImage.ctxCenterH"), icon: "AlignCenterHorizontal", onSelect: later(() => a.center(element.id, "h")) },
    { label: t("drawImage.ctxCenterV"), icon: "AlignCenterVertical", onSelect: later(() => a.center(element.id, "v")) },
    ...(element.type === "line" ? [{ label: t("drawImage.ctxAddPoint"), icon: "Plus", onSelect: later(() => a.addPoint(element.id)) } as MenuItem] : []),
    { type: "sep" },
    { label: t("drawImage.deleteElement"), icon: "Trash2", hint: "⌫", danger: true, onSelect: later(() => a.remove(element.id)) },
  ];
}

/* ------------------------------------------------------------------ */
/* canvas (empty area) menu                                            */
/* ------------------------------------------------------------------ */

export interface DrawCanvasMenuState {
  placing: boolean;
  showGrid: boolean;
  snap: boolean;
}

export interface DrawCanvasMenuActions {
  insert(type: DrawElementType, at: { x: number; y: number }): void;
  cancelPlacing(): void;
  toggleGrid(): void;
  toggleSnap(): void;
}

const INSERT_TOOLS: { type: DrawElementType; icon: string }[] = [
  { type: "rect", icon: "Square" },
  { type: "ellipse", icon: "Circle" },
  { type: "line", icon: "Minus" },
  { type: "star", icon: "Star" },
  { type: "text", icon: "Type" },
  { type: "image", icon: "Image" },
];

/** Clamps the right-click point onto the canvas so inserted elements land inside the doc. */
function clampToDoc(at: { x: number; y: number }, doc: DrawImageDoc): { x: number; y: number } {
  return {
    x: Math.min(Math.max(Math.round(at.x), 0), Math.max(0, doc.width - 1)),
    y: Math.min(Math.max(Math.round(at.y), 0), Math.max(0, doc.height - 1)),
  };
}

export function buildDrawCanvasMenu(
  at: { x: number; y: number },
  doc: DrawImageDoc,
  state: DrawCanvasMenuState,
  a: DrawCanvasMenuActions,
  t: TFunc,
): MenuItem[] {
  const items: MenuItem[] = [];
  if (state.placing) {
    items.push({ label: t("drawImage.ctxCancelPlacing"), icon: "X", hint: "Esc", onSelect: later(a.cancelPlacing) });
    items.push({ type: "sep" });
  } else {
    for (const tool of INSERT_TOOLS) {
      items.push({
        label: `${t("drawImage.insert")} ${t(`drawImage.types.${tool.type}`)}`,
        icon: tool.icon,
        onSelect: later(() => a.insert(tool.type, clampToDoc(at, doc))),
      });
    }
    items.push({ type: "sep" });
  }
  items.push({ label: t("drawImage.grid"), icon: "Grid2x2", checked: state.showGrid, onSelect: later(a.toggleGrid) });
  items.push({ label: t("drawImage.snap"), icon: "Magnet", checked: state.snap, onSelect: later(a.toggleSnap) });
  return items;
}
