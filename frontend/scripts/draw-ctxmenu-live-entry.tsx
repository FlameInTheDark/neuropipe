/**
 * Task 28 live entry — the Draw Image editor canvas CONTEXT MENUS over REAL
 * app components (DrawImageEditor, draw-image/*, ContextMenu, Modal, i18n)
 * with a demo doc exercising every menu variant:
 *  - rect/text/line on a normal layer          → element menu
 *  - star on a LOCKED layer                     → element menu + "Unlock layer"
 *  - hidden ellipse                             → element menu + "Show element"
 *  - empty canvas areas                         → insert/view menu
 * The modal opens on the "Open editor" button; window.__getDoc exposes the
 * serialized document so the harness script can assert menu side effects.
 */
import { createRoot } from "react-dom/client";
import { useEffect, useState } from "react";
import "../src/i18n";
import { ContextMenuProvider } from "../src/components/ContextMenu";
import { DrawImageEditor } from "../src/components/DrawImageEditor";

const DEMO_DOC = {
  version: 1,
  width: 600,
  height: 420,
  background: "#10161b",
  pins: [{ name: "title", type: "text", sample: "Quarterly report", default: "" }],
  layers: [
    { id: "layer_1", name: "Main", visible: true, opacity: 1, locked: false },
    { id: "layer_2", name: "Decoration (locked)", visible: true, opacity: 1, locked: true },
  ],
  elements: [
    {
      id: "el_rect",
      type: "rect",
      layerId: "layer_1",
      name: "Card",
      x: 40, y: 40, w: 520, h: 150,
      rotation: 0, opacity: 1, visible: true,
      visibility: { mode: "always", pin: "", op: "", value: "" },
      repeat: null,
      radius: 12,
      fill: { type: "solid", color: "#1d2b34" },
      stroke: { color: "#3b5866", width: 2, dash: [], cap: "butt", join: "miter" },
      points: [], starPoints: 5, innerRatio: 0.5,
      content: "", fontFamily: "inter", fontSize: 28, weight: 400, italic: false,
      color: "#f7f8f8", align: "left", valign: "top", lineHeight: 1.2, wrapWidth: -1,
      source: { kind: "url", value: "" }, fit: "fill", onMissing: "skip",
    },
    {
      id: "el_text",
      type: "text",
      layerId: "layer_1",
      name: "Headline",
      x: 64, y: 64, w: 400, h: 44,
      rotation: 0, opacity: 1, visible: true,
      visibility: { mode: "always", pin: "", op: "", value: "" },
      repeat: null,
      radius: 0,
      fill: { type: "solid", color: "#f7f8f8" },
      stroke: null,
      points: [], starPoints: 5, innerRatio: 0.5,
      content: "{{title}}", fontFamily: "inter", fontSize: 30, weight: 600, italic: false,
      color: "#f7f8f8", align: "left", valign: "top", lineHeight: 1.2, wrapWidth: -1,
      source: { kind: "url", value: "" }, fit: "fill", onMissing: "skip",
    },
    {
      id: "el_line",
      type: "line",
      layerId: "layer_1",
      name: "Divider",
      x: 64, y: 150, w: 460, h: 0,
      rotation: 0, opacity: 1, visible: true,
      visibility: { mode: "always", pin: "", op: "", value: "" },
      repeat: null,
      radius: 0,
      fill: { type: "solid", color: "#4ea7fc" },
      stroke: { color: "#4ea7fc", width: 3, dash: [], cap: "round", join: "round" },
      points: [{ x: 64, y: 150 }, { x: 524, y: 150 }], starPoints: 5, innerRatio: 0.5,
      content: "", fontFamily: "inter", fontSize: 28, weight: 400, italic: false,
      color: "#f7f8f8", align: "left", valign: "top", lineHeight: 1.2, wrapWidth: -1,
      source: { kind: "url", value: "" }, fit: "fill", onMissing: "skip",
    },
    {
      id: "el_ellipse",
      type: "ellipse",
      layerId: "layer_1",
      name: "Hidden ball",
      x: 470, y: 300, w: 90, h: 90,
      rotation: 0, opacity: 1, visible: false,
      visibility: { mode: "always", pin: "", op: "", value: "" },
      repeat: null,
      radius: 0,
      fill: { type: "solid", color: "#eb5757" },
      stroke: null,
      points: [], starPoints: 5, innerRatio: 0.5,
      content: "", fontFamily: "inter", fontSize: 28, weight: 400, italic: false,
      color: "#f7f8f8", align: "left", valign: "top", lineHeight: 1.2, wrapWidth: -1,
      source: { kind: "url", value: "" }, fit: "fill", onMissing: "skip",
    },
    {
      id: "el_star",
      type: "star",
      layerId: "layer_2",
      name: "Locked star",
      x: 110, y: 290, w: 110, h: 110,
      rotation: 0, opacity: 1, visible: true,
      visibility: { mode: "always", pin: "", op: "", value: "" },
      repeat: null,
      radius: 0,
      fill: { type: "solid", color: "#f0bf00" },
      stroke: null,
      points: [], starPoints: 5, innerRatio: 0.5,
      content: "", fontFamily: "inter", fontSize: 28, weight: 400, italic: false,
      color: "#f7f8f8", align: "left", valign: "top", lineHeight: 1.2, wrapWidth: -1,
      source: { kind: "url", value: "" }, fit: "fill", onMissing: "skip",
    },
  ],
};

declare global {
  interface Window {
    __getDoc: () => unknown;
    __docToScreen: (x: number, y: number) => { x: number; y: number } | null;
  }
}

function Harness() {
  const [value, setValue] = useState<unknown>(DEMO_DOC);

  useEffect(() => {
    window.__getDoc = () => value;
    // doc-space → screen-space helper: the stage canvas is the largest <canvas>
    // on screen (the inspector thumbnail is capped at 210px wide).
    window.__docToScreen = (x: number, y: number) => {
      let best: HTMLCanvasElement | null = null;
      let bestArea = 0;
      for (const canvas of Array.from(document.querySelectorAll("canvas"))) {
        const rect = canvas.getBoundingClientRect();
        const area = rect.width * rect.height;
        if (area > bestArea) {
          bestArea = area;
          best = canvas;
        }
      }
      if (!best) return null;
      const rect = best.getBoundingClientRect();
      const zoom = rect.width / DEMO_DOC.width;
      return { x: rect.left + x * zoom, y: rect.top + y * zoom };
    };
  }, [value]);

  return (
    <div className="min-h-screen bg-ink-1000 p-4" data-testid="draw-ctxmenu-harness">
      <div className="mx-auto max-w-[420px]">
        <ContextMenuProvider>
          <DrawImageEditor value={value} onChange={setValue} />
        </ContextMenuProvider>
      </div>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<Harness />);
