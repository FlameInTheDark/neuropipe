import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Badge, Button, IconButton } from "./ui";
import { Icon } from "./icons";
import { Modal } from "./primitives/Modal";
import {
  DrawElement,
  DrawElementType,
  DrawImageDoc,
  DrawLayer,
  DrawPin,
  isValidPinName,
  nextId,
  normalizeDrawImageDoc,
  createElement,
  serializeDrawImageDoc,
  sampleValuesFor,
  elementBounds,
} from "@/lib/draw-image";
import { ensureDrawFonts, type ImageResolver } from "@/lib/draw-image-render";
import { desktop } from "@/lib/bridge";
import { CanvasStage } from "./draw-image/CanvasStage";
import { LayersPanel } from "./draw-image/LayersPanel";
import { PropertiesPanel } from "./draw-image/PropertiesPanel";
import { PinsPanel } from "./draw-image/PinsPanel";
import { buildDrawCanvasMenu, buildDrawElementMenu } from "./draw-image/canvas-menus";
import type { MenuItem } from "./ContextMenu";

/* ------------------------------------------------------------------ */
/* inspector entry point                                               */
/* ------------------------------------------------------------------ */

export function DrawImageEditor({ value, onChange }: { value: unknown; onChange: (value: unknown) => void }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const doc = useMemo(() => normalizeDrawImageDoc(value), [value]);
  const thumbnailRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    if (!open) {
      let cancelled = false;
      ensureDrawFonts().then(() => {
        if (cancelled || !thumbnailRef.current) return;
        import("@/lib/draw-image-render").then(({ renderThumbnail }) => {
          if (!cancelled && thumbnailRef.current) {
            renderThumbnail(thumbnailRef.current, doc, sampleValuesFor(doc), 210, 118);
          }
        });
      });
      return () => {
        cancelled = true;
      };
    }
  }, [doc, open]);

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2 rounded-md border border-ink-700/70 bg-ink-850 px-2.5 py-[7px]">
        <span className="flex flex-col">
          <span className="text-[12.5px] font-medium text-fg">{t("drawImage.openEditor")}</span>
          <span className="text-[11px] text-fg-faint">
            {doc.width} × {doc.height} · {doc.elements.length} {t("drawImage.elementCount")}
          </span>
        </span>
        <Button variant="solid" icon="Image" onClick={() => setOpen(true)}>
          {t("drawImage.openEditorButton")}
        </Button>
      </div>
      <canvas
        ref={thumbnailRef}
        className="w-full max-w-[210px] cursor-pointer rounded-md border border-ink-700 bg-ink-950"
        style={{ aspectRatio: `${doc.width} / ${doc.height}` }}
        onClick={() => setOpen(true)}
        title={t("drawImage.openEditor")}
      />
      {open ? (
        <DrawImageModal
          value={value}
          onChange={onChange}
          onClose={() => setOpen(false)}
        />
      ) : null}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* modal                                                               */
/* ------------------------------------------------------------------ */

const HISTORY_LIMIT = 60;

function DrawImageModal({
  value,
  onChange,
  onClose,
}: {
  value: unknown;
  onChange: (value: unknown) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [doc, setDoc] = useState<DrawImageDoc>(() => normalizeDrawImageDoc(value));
  const docRef = useRef(doc);
  docRef.current = doc;
  const past = useRef<DrawImageDoc[]>([]);
  const future = useRef<DrawImageDoc[]>([]);
  const [historyVersion, setHistoryVersion] = useState(0);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [activeLayerId, setActiveLayerId] = useState<string>(() => doc.layers[0]?.id ?? "layer_1");
  const [placing, setPlacing] = useState<DrawElementType | null>(null);
  const [showGrid, setShowGrid] = useState(false);
  const [snap, setSnap] = useState(true);
  const [backendPreview, setBackendPreview] = useState(false);
  const [rightTab, setRightTab] = useState<"properties" | "inputs">("properties");
  const [status, setStatus] = useState<{ zoom: number; cursor: { x: number; y: number } | null }>({ zoom: 1, cursor: null });
  const [previewError, setPreviewError] = useState<string | null>(null);

  /* ---------------- history ---------------- */

  const beginHistory = useCallback(() => {
    past.current = [...past.current.slice(-(HISTORY_LIMIT - 1)), docRef.current];
    future.current = [];
    setHistoryVersion((v) => v + 1);
  }, []);

  const undo = useCallback(() => {
    if (past.current.length === 0) return;
    const previous = past.current[past.current.length - 1];
    past.current = past.current.slice(0, -1);
    future.current = [...future.current, docRef.current];
    setDoc(previous);
    docRef.current = previous;
    setHistoryVersion((v) => v + 1);
  }, []);

  const redo = useCallback(() => {
    if (future.current.length === 0) return;
    const next = future.current[future.current.length - 1];
    future.current = future.current.slice(0, -1);
    past.current = [...past.current, docRef.current];
    setDoc(next);
    docRef.current = next;
    setHistoryVersion((v) => v + 1);
  }, []);

  const mutate = useCallback(
    (fn: (current: DrawImageDoc) => DrawImageDoc, options?: { history?: boolean }) => {
      if (options?.history !== false) beginHistory();
      const next = fn(docRef.current);
      docRef.current = next;
      setDoc(next);
    },
    [beginHistory],
  );

  /* ---------------- external commit ---------------- */

  const committedRef = useRef(false);
  useEffect(() => {
    // skip the mount commit: opening the editor must not mark the graph dirty
    if (!committedRef.current) {
      committedRef.current = true;
      return;
    }
    onChange(serializeDrawImageDoc(doc));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [doc]);

  /* ---------------- element operations ---------------- */

  const patchElement = useCallback(
    (id: string, patch: Partial<DrawElement>) => {
      mutate(
        (current) => ({
          ...current,
          elements: current.elements.map((element) => (element.id === id ? normalizePatched(element, patch) : element)),
        }),
        { history: false },
      );
    },
    [mutate],
  );

  const patchElementWithHistory = useCallback(
    (id: string, patch: Partial<DrawElement>) => {
      mutate((current) => ({
        ...current,
        elements: current.elements.map((element) => (element.id === id ? normalizePatched(element, patch) : element)),
      }));
    },
    [mutate],
  );

  const addElement = useCallback(
    (type: DrawElementType, bounds?: { x: number; y: number; w: number; h: number } | { x: number; y: number }) => {
      let createdId = "";
      mutate((current) => {
        const nameCount = current.elements.filter((element) => element.type === type).length + 1;
        const element = createElement(type, activeLayerId, current, nameCount, bounds as { x: number; y: number; w: number; h: number } | undefined);
        createdId = element.id;
        return { ...current, elements: [...current.elements, element] };
      });
      if (createdId) {
        setSelectedId(createdId);
        setRightTab("properties");
      }
    },
    [activeLayerId, mutate],
  );

  const deleteElement = useCallback(
    (id: string) => {
      mutate((current) => ({ ...current, elements: current.elements.filter((element) => element.id !== id) }));
      if (selectedId === id) setSelectedId(null);
    },
    [mutate, selectedId],
  );

  const duplicateElement = useCallback(
    (id: string) => {
      let newId = "";
      mutate((current) => {
        const source = current.elements.find((element) => element.id === id);
        if (!source) return current;
        const used = new Set(current.elements.map((element) => element.id));
        newId = nextId("el", used);
        const clone: DrawElement = {
          ...source,
          id: newId,
          name: `${source.name} ${t("drawImage.copySuffix")}`,
          x: source.x + 16,
          y: source.y + 16,
          points: source.points.map((point) => ({ x: point.x + 16, y: point.y + 16 })),
        };
        const index = current.elements.findIndex((element) => element.id === id);
        const elements = [...current.elements];
        elements.splice(index + 1, 0, clone);
        return { ...current, elements };
      });
      if (newId) setSelectedId(newId);
    },
    [mutate, t],
  );

  const moveElement = useCallback(
    (id: string, direction: -1 | 1) => {
      mutate((current) => {
        const index = current.elements.findIndex((element) => element.id === id);
        const target = index + direction;
        if (index < 0 || target < 0 || target >= current.elements.length) return current;
        const elements = [...current.elements];
        const [item] = elements.splice(index, 1);
        elements.splice(target, 0, item);
        return { ...current, elements };
      });
    },
    [mutate],
  );

  const reorderElement = useCallback(
    (id: string, target: "front" | "back") => {
      mutate((current) => {
        const index = current.elements.findIndex((element) => element.id === id);
        if (index < 0) return current;
        if (target === "front" && index === current.elements.length - 1) return current;
        if (target === "back" && index === 0) return current;
        const elements = [...current.elements];
        const [item] = elements.splice(index, 1);
        if (target === "front") elements.push(item);
        else elements.unshift(item);
        return { ...current, elements };
      });
    },
    [mutate],
  );

  /* ---------------- layer operations ---------------- */

  const patchLayer = useCallback(
    (id: string, patch: Partial<DrawLayer>) => {
      mutate((current) => ({
        ...current,
        layers: current.layers.map((layer) => (layer.id === id ? { ...layer, ...patch } : layer)),
      }));
    },
    [mutate],
  );

  const addLayer = useCallback(() => {
    mutate((current) => {
      const used = new Set(current.layers.map((layer) => layer.id));
      const id = nextId("layer", used);
      const layer: DrawLayer = { id, name: `${t("drawImage.layerName")} ${current.layers.length + 1}`, visible: true, opacity: 1, locked: false };
      return { ...current, layers: [...current.layers, layer] };
    });
  }, [mutate, t]);

  const deleteLayer = useCallback(
    (id: string) => {
      mutate((current) => {
        if (current.layers.length <= 1) return current;
        const fallback = current.layers.find((layer) => layer.id !== id)!;
        return {
          ...current,
          layers: current.layers.filter((layer) => layer.id !== id),
          elements: current.elements.map((element) => (element.layerId === id ? { ...element, layerId: fallback.id } : element)),
        };
      });
      if (activeLayerId === id) {
        const next = docRef.current.layers[0];
        if (next) setActiveLayerId(next.id);
      }
    },
    [activeLayerId, mutate],
  );

  const duplicateLayer = useCallback(
    (id: string) => {
      let newId = "";
      mutate((current) => {
        const source = current.layers.find((layer) => layer.id === id);
        if (!source) return current;
        const usedLayerIds = new Set(current.layers.map((layer) => layer.id));
        newId = nextId("layer", usedLayerIds);
        const usedElementIds = new Set(current.elements.map((element) => element.id));
        const clones = current.elements
          .filter((element) => element.layerId === id)
          .map((element) => ({ ...element, id: nextId("el", usedElementIds), layerId: newId, name: `${element.name} ${t("drawImage.copySuffix")}` }));
        const index = current.layers.findIndex((layer) => layer.id === id);
        const layers = [...current.layers];
        layers.splice(index + 1, 0, { ...source, id: newId, name: `${source.name} ${t("drawImage.copySuffix")}` });
        return { ...current, layers, elements: [...current.elements, ...clones] };
      });
      if (newId) setActiveLayerId(newId);
    },
    [mutate, t],
  );

  const moveLayer = useCallback(
    (id: string, direction: -1 | 1) => {
      mutate((current) => {
        const index = current.layers.findIndex((layer) => layer.id === id);
        const target = index + direction;
        if (index < 0 || target < 0 || target >= current.layers.length) return current;
        const layers = [...current.layers];
        const [item] = layers.splice(index, 1);
        layers.splice(target, 0, item);
        return { ...current, layers };
      });
    },
    [mutate],
  );

  /* ---------------- pin operations ---------------- */

  const addPin = useCallback(() => {
    mutate((current) => {
      const used = new Set(current.pins.map((pin) => pin.name));
      let index = current.pins.length + 1;
      let name = `value${index}`;
      while (used.has(name)) {
        index += 1;
        name = `value${index}`;
      }
      return { ...current, pins: [...current.pins, { name, type: "text", sample: "", default: "" }] };
    });
  }, [mutate]);

  const patchPin = useCallback(
    (name: string, patch: Partial<DrawPin>) => {
      mutate((current) => ({
        ...current,
        pins: current.pins.map((pin) => (pin.name === name ? { ...pin, ...patch } : pin)),
      }));
    },
    [mutate],
  );

  const deletePin = useCallback(
    (name: string) => {
      mutate((current) => ({
        ...current,
        pins: current.pins.filter((pin) => pin.name !== name),
        elements: current.elements.map((element) => {
          const next = { ...element };
          if (next.visibility.pin === name) next.visibility = { mode: "always", pin: "", op: "", value: "" };
          if (next.repeat?.pin === name) next.repeat = null;
          if (next.type === "image" && next.source.kind === "pin" && next.source.value === name) {
            next.source = { kind: "url", value: "" };
          }
          if (next.type === "text") {
            next.content = next.content.split(`{{${name}}}`).join("");
          }
          return next;
        }),
      }));
    },
    [mutate],
  );

  const renamePin = useCallback(
    (oldName: string, newName: string) => {
      mutate((current) => ({
        ...current,
        pins: current.pins.map((pin) => (pin.name === oldName ? { ...pin, name: newName } : pin)),
        elements: current.elements.map((element) => {
          const next = { ...element };
          if (next.visibility.pin === oldName) next.visibility = { ...next.visibility, pin: newName };
          if (next.repeat?.pin === oldName) next.repeat = { ...next.repeat, pin: newName };
          if (next.type === "image" && next.source.kind === "pin" && next.source.value === oldName) {
            next.source = { kind: "pin", value: newName };
          }
          if (next.type === "text") {
            next.content = next.content.split(`{{${oldName}}}`).join(`{{${newName}}}`);
          }
          return next;
        }),
      }));
    },
    [mutate],
  );

  const patchDoc = useCallback(
    (patch: Partial<Pick<DrawImageDoc, "width" | "height" | "background">>) => {
      mutate((current) => ({ ...current, ...patch }));
    },
    [mutate],
  );

  /* ---------------- context menus ---------------- */

  const elementCtx = useCallback(
    (id: string): MenuItem[] => {
      const element = docRef.current.elements.find((item) => item.id === id);
      if (!element) return [];
      return buildDrawElementMenu(
        element,
        docRef.current,
        {
          editProperties: (targetId) => {
            setSelectedId(targetId);
            setRightTab("properties");
          },
          duplicate: (targetId) => duplicateElement(targetId),
          toggleVisible: (targetId, visible) => patchElementWithHistory(targetId, { visible }),
          moveZ: (targetId, direction) => moveElement(targetId, direction),
          reorder: (targetId, target) => reorderElement(targetId, target),
          rotate: (targetId, degrees) => {
            const source = docRef.current.elements.find((item) => item.id === targetId);
            if (!source) return;
            const rotation = ((((source.rotation + degrees) % 360) + 360) % 360);
            patchElementWithHistory(targetId, { rotation: rotation > 180 ? rotation - 360 : rotation });
          },
          center: (targetId, axis) => {
            const source = docRef.current.elements.find((item) => item.id === targetId);
            if (!source) return;
            const current = docRef.current;
            const bounds = elementBounds(source);
            if (axis === "h") {
              const dx = Math.round((current.width - bounds.w) / 2 - bounds.x);
              if (source.type === "line") {
                patchElementWithHistory(targetId, lineElementPatch(source.points.map((p) => ({ x: p.x + dx, y: p.y }))));
              } else {
                patchElementWithHistory(targetId, { x: source.x + dx });
              }
            } else {
              const dy = Math.round((current.height - bounds.h) / 2 - bounds.y);
              if (source.type === "line") {
                patchElementWithHistory(targetId, lineElementPatch(source.points.map((p) => ({ x: p.x, y: p.y + dy }))));
              } else {
                patchElementWithHistory(targetId, { y: source.y + dy });
              }
            }
          },
          addPoint: (targetId) => {
            const source = docRef.current.elements.find((item) => item.id === targetId);
            if (!source || source.type !== "line" || source.points.length === 0) return;
            const points = source.points;
            const last = points[points.length - 1];
            const prev = points[points.length - 2] ?? { x: last.x - 40, y: last.y };
            let dx = last.x - prev.x;
            let dy = last.y - prev.y;
            const length = Math.hypot(dx, dy) || 1;
            dx = (dx / length) * 40;
            dy = (dy / length) * 40;
            patchElementWithHistory(
              targetId,
              lineElementPatch([...points, { x: Math.round(last.x + dx), y: Math.round(last.y + dy) }]),
            );
          },
          remove: (targetId) => deleteElement(targetId),
          unlockLayer: (layerId) => patchLayer(layerId, { locked: false }),
          showLayer: (layerId) => patchLayer(layerId, { visible: true }),
        },
        t,
      );
    },
    [t, duplicateElement, moveElement, reorderElement, patchElementWithHistory, patchLayer, deleteElement],
  );

  const canvasCtx = useCallback(
    (at: { x: number; y: number }): MenuItem[] =>
      buildDrawCanvasMenu(
        at,
        docRef.current,
        { placing: placing !== null, showGrid, snap },
        {
          insert: (type, point) => addElement(type, point),
          cancelPlacing: () => setPlacing(null),
          toggleGrid: () => setShowGrid((v) => !v),
          toggleSnap: () => setSnap((v) => !v),
        },
        t,
      ),
    [t, placing, showGrid, snap, addElement],
  );

  /* ---------------- keyboard shortcuts ---------------- */

  const selected = doc.elements.find((element) => element.id === selectedId) ?? null;

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable)) return;
      const meta = event.metaKey || event.ctrlKey;
      if (meta && event.key.toLowerCase() === "z" && !event.shiftKey) {
        event.preventDefault();
        undo();
        return;
      }
      if ((meta && event.key.toLowerCase() === "z" && event.shiftKey) || (meta && event.key.toLowerCase() === "y")) {
        event.preventDefault();
        redo();
        return;
      }
      if (meta && event.key.toLowerCase() === "d") {
        event.preventDefault();
        if (selectedId) duplicateElement(selectedId);
        return;
      }
      if (event.key === "Escape") {
        // capture phase: consume Escape when there is something to cancel so
        // the modal shell does not close underneath the interaction
        if (placing) {
          event.stopPropagation();
          setPlacing(null);
          return;
        }
        if (selectedId) {
          event.stopPropagation();
          setSelectedId(null);
          return;
        }
        return;
      }
      if ((event.key === "Delete" || event.key === "Backspace") && selectedId) {
        event.preventDefault();
        deleteElement(selectedId);
        return;
      }
      if (selected && event.key.startsWith("Arrow")) {
        event.preventDefault();
        const step = event.shiftKey ? 10 : 1;
        const dx = event.key === "ArrowLeft" ? -step : event.key === "ArrowRight" ? step : 0;
        const dy = event.key === "ArrowUp" ? -step : event.key === "ArrowDown" ? step : 0;
        if (selected.type === "line") {
          patchElement(selected.id, { points: selected.points.map((point) => ({ x: point.x + dx, y: point.y + dy })) });
        } else {
          patchElement(selected.id, { x: selected.x + dx, y: selected.y + dy });
        }
      }
    };
    window.addEventListener("keydown", handler, true);
    return () => window.removeEventListener("keydown", handler, true);
  }, [undo, redo, selectedId, selected, placing, deleteElement, duplicateElement, patchElement]);

  /* ---------------- image resolver + backend preview ---------------- */

  const imageResolver = useCallback<ImageResolver>(async (kind, value) => {
    const resolved = kind === "url" ? "url" : "path";
    return desktop.drawImageLoadImageSource(resolved, value);
  }, []);

  const [backendDataUrl, setBackendDataUrl] = useState<string | null>(null);
  const values = useMemo(() => sampleValuesFor(doc), [doc]);

  useEffect(() => {
    if (!backendPreview) return;
    let cancelled = false;
    const timer = setTimeout(() => {
      const docJSON = JSON.stringify(serializeDrawImageDoc(doc));
      desktop
        .drawImagePreview(docJSON, JSON.stringify(values))
        .then((base64) => {
          if (!cancelled) {
            setBackendDataUrl(`data:image/png;base64,${base64}`);
            setPreviewError(null);
          }
        })
        .catch((error) => {
          if (!cancelled) {
            setBackendDataUrl(null);
            setPreviewError(String(error));
          }
        });
    }, 250);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [backendPreview, doc, values]);

  /* ---------------- render ---------------- */

  const canUndo = past.current.length > 0;
  const canRedo = future.current.length > 0;
  void historyVersion;

  const insertTools: { type: DrawElementType; icon: string; label: string }[] = [
    { type: "rect", icon: "Square", label: t("drawImage.types.rect") },
    { type: "ellipse", icon: "Circle", label: t("drawImage.types.ellipse") },
    { type: "line", icon: "Minus", label: t("drawImage.types.line") },
    { type: "star", icon: "Star", label: t("drawImage.types.star") },
    { type: "text", icon: "Type", label: t("drawImage.types.text") },
    { type: "image", icon: "Image", label: t("drawImage.types.image") },
  ];

  return (
    <Modal
      title={t("drawImage.editorTitle")}
      icon="Image"
      size="full"
      onClose={onClose}
      headerExtra={
        <Badge tone="muted">
          {doc.width} × {doc.height}
        </Badge>
      }
      bodyClassName="min-h-0 flex-1 overflow-hidden p-0"
      footer={
        <div className="flex w-full items-center gap-3">
          <span className="font-mono text-[10.5px] text-fg-faint">
            {status.cursor ? `X ${status.cursor.x} · Y ${status.cursor.y}` : `${doc.width} × ${doc.height}`}
          </span>
          <span className="font-mono text-[10.5px] text-fg-faint">{Math.round(status.zoom * 100)}%</span>
          <span className="ml-auto text-[10.5px] text-fg-faint">{t("drawImage.footerHint")}</span>
          <Button variant="primary" icon="Check" onClick={onClose}>
            {t("drawImage.done")}
          </Button>
        </div>
      }
    >
      {/* toolbar */}
      <div className="flex shrink-0 flex-wrap items-center gap-1.5 border-b border-seam px-3 py-1.5">
        <div className="flex items-center gap-0.5 rounded-md border border-ink-700 bg-ink-900/60 p-0.5">
          {insertTools.map((tool) => (
            <button
              key={tool.type}
              title={`${t("drawImage.insert")} ${tool.label}`}
              onClick={() => setPlacing((current) => (current === tool.type ? null : tool.type))}
              className={`grid h-6 w-6 place-items-center rounded transition ${
                placing === tool.type ? "bg-ink-700 text-fg" : "text-fg-subtle hover:bg-ink-800 hover:text-fg"
              }`}
            >
              <Icon name={tool.icon} className="h-3.5 w-3.5" />
            </button>
          ))}
        </div>
        <div className="flex items-center gap-0.5">
          <IconButton icon="Undo2" label={t("drawImage.undo")} onClick={undo} size="sm" className={canUndo ? "" : "opacity-40"} />
          <IconButton icon="Redo2" label={t("drawImage.redo")} onClick={redo} size="sm" className={canRedo ? "" : "opacity-40"} />
        </div>
        <div className="flex items-center gap-0.5">
          <IconButton icon="Grid2x2" label={t("drawImage.grid")} active={showGrid} onClick={() => setShowGrid((v) => !v)} size="sm" />
          <IconButton icon="Magnet" label={t("drawImage.snap")} active={snap} onClick={() => setSnap((v) => !v)} size="sm" />
        </div>
        <div className="flex items-center gap-1 rounded-md border border-ink-700 bg-ink-900/60 px-1.5 py-0.5">
          <span className="text-[10px] text-fg-faint">{t("drawImage.canvas")}</span>
          <input
            type="number"
            value={doc.width}
            min={1}
            max={8192}
            onChange={(event) => {
              const width = Number(event.target.value);
              if (Number.isFinite(width) && width >= 1 && width <= 8192) patchDoc({ width: Math.round(width) });
            }}
            className="w-[58px] rounded border border-ink-700 bg-ink-850 px-1 py-0.5 text-right text-[11px] text-fg outline-none focus:border-ink-500"
          />
          <span className="text-[10px] text-fg-faint">×</span>
          <input
            type="number"
            value={doc.height}
            min={1}
            max={8192}
            onChange={(event) => {
              const height = Number(event.target.value);
              if (Number.isFinite(height) && height >= 1 && height <= 8192) patchDoc({ height: Math.round(height) });
            }}
            className="w-[58px] rounded border border-ink-700 bg-ink-850 px-1 py-0.5 text-right text-[11px] text-fg outline-none focus:border-ink-500"
          />
        </div>
        <div className="ml-auto flex items-center gap-1.5">
          <Button
            variant="ghost"
            icon="Zap"
            className={backendPreview ? "bg-ink-700 text-fg" : ""}
            onClick={() => setBackendPreview((v) => !v)}
          >
            {t("drawImage.backendPreview")}
          </Button>
        </div>
      </div>

      {/* main area */}
      <div className="grid h-full min-h-0 grid-cols-[230px_1fr_300px]">
        {/* left */}
        <aside className="muted-scroll min-h-0 overflow-y-auto border-r border-seam bg-ink-950/40">
          <LayersPanel
            doc={doc}
            activeLayerId={activeLayerId}
            selectedElementId={selectedId}
            onActiveLayer={setActiveLayerId}
            onSelectElement={setSelectedId}
            onPatchLayer={patchLayer}
            onAddLayer={addLayer}
            onDeleteLayer={deleteLayer}
            onDuplicateLayer={duplicateLayer}
            onMoveLayer={moveLayer}
            onPatchElement={patchElementWithHistory}
            onDeleteElement={deleteElement}
            onDuplicateElement={duplicateElement}
            onMoveElement={moveElement}
          />
        </aside>

        {/* center */}
        <div className="relative min-h-0">
          {backendPreview ? (
            <div className="grid h-full min-h-0 grid-cols-2">
              <div className="min-h-0 border-r border-seam">
                <StageLabel text={t("drawImage.editorCanvas")} />
                <div className="h-full pt-6">
                  <CanvasStage
                    doc={doc}
                    selectedId={selectedId}
                    placing={placing}
                    showGrid={showGrid}
                    snap={snap}
                    onSelect={setSelectedId}
                    onBeginHistory={beginHistory}
                    onPatchElement={patchElement}
                    onPlaceElement={(type, bounds) => addElement(type, bounds)}
                    onPlacingDone={() => setPlacing(null)}
                    imageResolver={imageResolver}
                    onStatus={setStatus}
                    elementCtx={elementCtx}
                    canvasCtx={canvasCtx}
                  />
                </div>
              </div>
              <div className="min-h-0">
                <StageLabel text={t("drawImage.backendCanvas")} accent />
                <div className="grid h-full place-items-center overflow-auto bg-ink-950 p-4 pt-10">
                  {previewError ? (
                    <div className="max-w-sm rounded-md border border-danger/40 bg-danger/10 p-3 text-[11.5px] leading-5 text-danger-fg">
                      {previewError}
                    </div>
                  ) : backendDataUrl ? (
                    <img src={backendDataUrl} alt={t("drawImage.backendCanvas")} className="max-h-full max-w-full rounded-[2px] object-contain shadow-[0_0_0_1px_rgba(255,255,255,0.08)]" />
                  ) : (
                    <span className="text-[11px] text-fg-faint">{t("drawImage.rendering")}</span>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <CanvasStage
              doc={doc}
              selectedId={selectedId}
              placing={placing}
              showGrid={showGrid}
              snap={snap}
              onSelect={setSelectedId}
              onBeginHistory={beginHistory}
              onPatchElement={patchElement}
              onPlaceElement={(type, bounds) => addElement(type, bounds)}
              onPlacingDone={() => setPlacing(null)}
              imageResolver={imageResolver}
              onStatus={setStatus}
              elementCtx={elementCtx}
              canvasCtx={canvasCtx}
            />
          )}
        </div>

        {/* right */}
        <aside className="flex min-h-0 flex-col border-l border-seam bg-ink-950/40">
          <div className="flex shrink-0 items-center gap-1 border-b border-seam p-1.5">
            {(["properties", "inputs"] as const).map((tab) => (
              <button
                key={tab}
                onClick={() => setRightTab(tab)}
                className={`h-6 flex-1 rounded-md px-2 text-[11px] font-medium transition ${
                  rightTab === tab ? "bg-ink-700 text-fg" : "text-fg-subtle hover:bg-ink-800 hover:text-fg"
                }`}
              >
                {t(tab === "properties" ? "drawImage.propertiesTab" : "drawImage.inputsTab")}
              </button>
            ))}
          </div>
          <div className="min-h-0 flex-1">
            {rightTab === "properties" ? (
              <PropertiesPanel doc={doc} element={selected} onPatchElement={patchElementWithHistory} onPatchDoc={patchDoc} />
            ) : (
              <PinsPanel
                doc={doc}
                onAddPin={addPin}
                onPatchPin={patchPin}
                onDeletePin={deletePin}
                onRenamePin={renamePin}
              />
            )}
          </div>
        </aside>
      </div>
    </Modal>
  );
}

function StageLabel({ text, accent }: { text: string; accent?: boolean }) {
  return (
    <span
      className={`absolute left-2 top-1.5 z-10 rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${
        accent ? "border-teal/40 bg-teal/10 text-teal" : "border-ink-650 bg-ink-900/90 text-fg-faint"
      }`}
    >
      {text}
    </span>
  );
}

/** Keeps a line element's bounding box in sync with its points (mirrors CanvasStage gesture logic). */
function lineElementPatch(points: { x: number; y: number }[]): Partial<DrawElement> {
  const xs = points.map((p) => p.x);
  const ys = points.map((p) => p.y);
  return {
    points,
    x: Math.min(...xs),
    y: Math.min(...ys),
    w: Math.max(...xs) - Math.min(...xs),
    h: Math.max(...ys) - Math.min(...ys),
  };
}

/** Re-normalizes clamped fields when patches arrive from the editor. */
function normalizePatched(element: DrawElement, patch: Partial<DrawElement>): DrawElement {
  const merged: DrawElement = { ...element, ...patch };
  merged.opacity = Math.min(1, Math.max(0, merged.opacity));
  merged.rotation = Math.min(360, Math.max(-360, merged.rotation));
  merged.fontSize = Math.min(512, Math.max(1, merged.fontSize));
  merged.wrapWidth = Math.min(8192, Math.max(-1, merged.wrapWidth));
  if (merged.repeat && (!isValidPinName(merged.repeat.pin) && !merged.repeat.pin)) merged.repeat = null;
  if (merged.repeat && !isValidPinName(merged.repeat.pin)) merged.repeat = null;
  return merged;
}
