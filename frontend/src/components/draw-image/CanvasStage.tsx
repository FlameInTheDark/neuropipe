import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useCtxMenu, type MenuItem } from "../ContextMenu";
import { DrawElement, DrawElementType, DrawImageDoc, elementBounds, sampleValuesFor } from "@/lib/draw-image";
import { ensureDrawFonts, preloadDocImages, renderDrawImageDoc, type ImageResolver } from "@/lib/draw-image-render";

type Handle = "nw" | "n" | "ne" | "e" | "se" | "s" | "sw" | "w";

interface Gesture {
  kind: "move" | "resize" | "rotate" | "create" | "endpoint" | "pan";
  handle?: Handle;
  endpoint?: number;
  startDoc: { x: number; y: number };
  origin: DrawElement | null;
  createType?: DrawElementType;
  createBounds?: { x: number; y: number; w: number; h: number };
  startPan?: { x: number; y: number };
  startAngle?: number;
  startRotation?: number;
}

export interface CanvasStageProps {
  doc: DrawImageDoc;
  selectedId: string | null;
  placing: DrawElementType | null;
  showGrid: boolean;
  snap: boolean;
  onSelect(id: string | null): void;
  onBeginHistory(): void;
  onPatchElement(id: string, patch: Partial<DrawElement>): void;
  onPlaceElement(type: DrawElementType, bounds: { x: number; y: number; w: number; h: number } | { x: number; y: number }): void;
  onPlacingDone(): void;
  imageResolver: ImageResolver;
  onStatus?(status: { zoom: number; cursor: { x: number; y: number } | null }): void;
  /** context menu items for the element under the cursor (built by the editor shell) */
  elementCtx?(id: string): MenuItem[];
  /** context menu items for empty canvas areas (insert + toggles; view items are appended here) */
  canvasCtx?(at: { x: number; y: number }): MenuItem[];
}

const HANDLE_SIZE = 9;
const SNAP_DISTANCE = 6;

export function CanvasStage(props: CanvasStageProps) {
  const { t } = useTranslation();
  const { doc, selectedId, placing } = props;
  const openCtxMenu = useCtxMenu();
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [zoom, setZoom] = useState(0.5);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const gestureRef = useRef<Gesture | null>(null);
  const [liveOverride, setLiveOverride] = useState<Partial<DrawElement> | null>(null);
  const [spaceHeld, setSpaceHeld] = useState(false);
  const [imageVersion, setImageVersion] = useState(0);
  const values = useMemo(() => sampleValuesFor(doc), [doc]);

  const selected = doc.elements.find((element) => element.id === selectedId) ?? null;

  /* ---------------- view helpers ---------------- */

  const fit = useCallback(() => {
    const container = containerRef.current;
    if (!container) return;
    const padding = 48;
    const scale = Math.min(
      (container.clientWidth - padding) / doc.width,
      (container.clientHeight - padding) / doc.height,
      4,
    );
    const nextZoom = Math.max(0.05, scale);
    setZoom(nextZoom);
    setPan({
      x: (container.clientWidth - doc.width * nextZoom) / 2,
      y: (container.clientHeight - doc.height * nextZoom) / 2,
    });
  }, [doc.width, doc.height]);

  /** zooms while keeping the viewport center anchored */
  const zoomTo = useCallback(
    (nextZoom: number) => {
      const container = containerRef.current;
      if (!container) return;
      const clamped = Math.min(8, Math.max(0.05, nextZoom));
      const cx = container.clientWidth / 2;
      const cy = container.clientHeight / 2;
      setPan({
        x: cx - ((cx - pan.x) / zoom) * clamped,
        y: cy - ((cy - pan.y) / zoom) * clamped,
      });
      setZoom(clamped);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [pan, zoom],
  );

  useEffect(() => {
    fit();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [doc.width, doc.height]);

  useEffect(() => {
    props.onStatus?.({ zoom, cursor: null });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [zoom]);

  /* ---------------- fonts + images ---------------- */

  useEffect(() => {
    let cancelled = false;
    ensureDrawFonts().then(() => {
      if (!cancelled) setImageVersion((v) => v + 1);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    preloadDocImages(doc, values, props.imageResolver).then(() => {
      if (!cancelled) setImageVersion((v) => v + 1);
    });
    return () => {
      cancelled = true;
    };
  }, [doc, values, props.imageResolver]);

  /* ---------------- rendering ---------------- */

  const renderDoc = useMemo(() => {
    if (!selected || !liveOverride) return doc;
    return {
      ...doc,
      elements: doc.elements.map((element) => (element.id === selected.id ? { ...element, ...liveOverride } : element)),
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [doc, selected?.id, liveOverride]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    const renderScale = Math.min(Math.max(window.devicePixelRatio || 1, zoom), 4);
    canvas.width = Math.round(doc.width * renderScale);
    canvas.height = Math.round(doc.height * renderScale);
    canvas.style.width = `${doc.width * zoom}px`;
    canvas.style.height = `${doc.height * zoom}px`;
    ctx.setTransform(renderScale, 0, 0, renderScale, 0, 0);
    renderDrawImageDoc(ctx, renderDoc, values);
  }, [renderDoc, values, zoom, imageVersion]);

  /* ---------------- coordinate transforms ---------------- */

  const toDoc = useCallback(
    (clientX: number, clientY: number): { x: number; y: number } => {
      const container = containerRef.current;
      if (!container) return { x: 0, y: 0 };
      const rect = container.getBoundingClientRect();
      return {
        x: (clientX - rect.left - pan.x) / zoom,
        y: (clientY - rect.top - pan.y) / zoom,
      };
    },
    [pan, zoom],
  );

  const rotatePoint = (px: number, py: number, cx: number, cy: number, angleDeg: number) => {
    if (angleDeg === 0) return { x: px, y: py };
    const rad = (-angleDeg * Math.PI) / 180;
    const dx = px - cx;
    const dy = py - cy;
    return { x: cx + dx * Math.cos(rad) - dy * Math.sin(rad), y: cy + dx * Math.sin(rad) + dy * Math.cos(rad) };
  };

  /* ---------------- hit testing ---------------- */

  /**
   * Topmost element whose rotated bounds contain the point. With `includeAll`
   * invisible elements and elements on hidden/locked layers are hit too — used
   * by the context menu so right-click can reveal and unlock them.
   */
  const elementAt = useCallback(
    (point: { x: number; y: number }, includeAll = false): DrawElement | null => {
      const lockedLayers = new Set(
        doc.layers.filter((layer) => (includeAll ? false : layer.locked || !layer.visible)).map((layer) => layer.id),
      );
      for (let i = doc.elements.length - 1; i >= 0; i--) {
        const element = doc.elements[i];
        if ((!element.visible && !includeAll) || lockedLayers.has(element.layerId)) continue;
        const bounds = elementBounds(element);
        const local = rotatePoint(point.x, point.y, bounds.x + bounds.w / 2, bounds.y + bounds.h / 2, element.rotation);
        const pad = 2 / zoom;
        if (
          local.x >= bounds.x - pad &&
          local.x <= bounds.x + bounds.w + pad &&
          local.y >= bounds.y - pad &&
          local.y <= bounds.y + bounds.h + pad
        ) {
          // text/line elements: tighter check — accept for now (bbox hit)
          return element;
        }
      }
      return null;
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [doc, zoom],
  );

  /* ---------------- pointer interactions ---------------- */

  const beginGesture = (gesture: Gesture) => {
    gestureRef.current = gesture;
  };

  const onStagePointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (event.button === 1 || spaceHeld) {
      beginGesture({ kind: "pan", startDoc: { x: 0, y: 0 }, origin: null, startPan: { ...pan } });
      (event.target as HTMLElement).setPointerCapture(event.pointerId);
      return;
    }
    if (event.button !== 0) return;
    const point = toDoc(event.clientX, event.clientY);
    if (placing) {
      beginGesture({ kind: "create", startDoc: point, origin: null, createType: placing, createBounds: { x: point.x, y: point.y, w: 0, h: 0 } });
      (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
      return;
    }
    const hit = elementAt(point);
    if (hit) {
      props.onSelect(hit.id);
      props.onBeginHistory();
      beginGesture({ kind: "move", startDoc: point, origin: { ...hit } });
      (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
    } else {
      props.onSelect(null);
    }
  };

  const onStagePointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    const point = toDoc(event.clientX, event.clientY);
    props.onStatus?.({ zoom, cursor: { x: Math.round(point.x), y: Math.round(point.y) } });
    const gesture = gestureRef.current;
    if (!gesture) return;

    switch (gesture.kind) {
      case "pan": {
        setPan((prev) => ({ x: prev.x + event.movementX, y: prev.y + event.movementY }));
        break;
      }
      case "create": {
        const bounds = {
          x: Math.min(gesture.startDoc.x, point.x),
          y: Math.min(gesture.startDoc.y, point.y),
          w: Math.abs(point.x - gesture.startDoc.x),
          h: Math.abs(point.y - gesture.startDoc.y),
        };
        gesture.createBounds = bounds;
        setLiveOverride({ x: bounds.x, y: bounds.y, w: bounds.w, h: bounds.h, type: gesture.createType });
        break;
      }
      case "move": {
        if (!gesture.origin) break;
        const dx = point.x - gesture.startDoc.x;
        const dy = point.y - gesture.startDoc.y;
        let nx = gesture.origin.x + dx;
        let ny = gesture.origin.y + dy;
        if (gesture.origin.type === "line") {
          const points = gesture.origin.points.map((p) => ({ x: p.x + dx, y: p.y + dy }));
          if (props.snap) {
            const snapped = snapBox(points[0]?.x ?? nx, points[0]?.y ?? ny, 0, 0, doc);
            const sdx = snapped.x - (gesture.origin.points[0]?.x ?? 0);
            const sdy = snapped.y - (gesture.origin.points[0]?.y ?? 0);
            if (sdx !== 0 || sdy !== 0) {
              setLiveOverride({ points: points.map((p) => ({ x: p.x + sdx, y: p.y + sdy })) });
              break;
            }
          }
          setLiveOverride({ points });
          break;
        }
        if (props.snap) {
          const snapped = snapBox(nx, ny, gesture.origin.w, gesture.origin.h, doc);
          nx = snapped.x;
          ny = snapped.y;
        }
        setLiveOverride({ x: Math.round(nx), y: Math.round(ny) });
        break;
      }
      case "resize": {
        if (!gesture.origin || !gesture.handle) break;
        const origin = gesture.origin;
        const cx = origin.x + origin.w / 2;
        const cy = origin.y + origin.h / 2;
        const local = rotatePoint(point.x, point.y, cx, cy, origin.rotation);
        let left = origin.x;
        let top = origin.y;
        let right = origin.x + origin.w;
        let bottom = origin.y + origin.h;
        const handle = gesture.handle;
        if (handle.includes("w")) left = local.x;
        if (handle.includes("e")) right = local.x;
        if (handle.includes("n")) top = local.y;
        if (handle.includes("s")) bottom = local.y;
        const x = Math.min(left, right);
        const y = Math.min(top, bottom);
        const w = Math.abs(right - left);
        const h = Math.abs(bottom - top);
        setLiveOverride({ x: Math.round(x), y: Math.round(y), w: Math.round(w), h: Math.round(h) });
        break;
      }
      case "rotate": {
        if (!gesture.origin) break;
        const origin = gesture.origin;
        const cx = origin.x + origin.w / 2;
        const cy = origin.y + origin.h / 2;
        const angle = (Math.atan2(point.y - cy, point.x - cx) * 180) / Math.PI;
        let rotation = angle + 90;
        if (event.shiftKey) rotation = Math.round(rotation / 15) * 15;
        rotation = ((rotation % 360) + 360) % 360;
        if (rotation > 180) rotation -= 360;
        setLiveOverride({ rotation: Math.round(rotation * 10) / 10 });
        break;
      }
      case "endpoint": {
        if (!gesture.origin || gesture.endpoint === undefined) break;
        const points = gesture.origin.points.map((p, index) =>
          index === gesture.endpoint ? { x: Math.round(point.x), y: Math.round(point.y) } : p,
        );
        setLiveOverride({ points });
        break;
      }
    }
  };

  const onStagePointerUp = () => {
    const gesture = gestureRef.current;
    gestureRef.current = null;
    if (!gesture) return;
    if (gesture.kind === "pan") return;
    if (gesture.kind === "create" && gesture.createType) {
      const bounds = gesture.createBounds ?? { x: gesture.startDoc.x, y: gesture.startDoc.y, w: 0, h: 0 };
      setLiveOverride(null);
      if (bounds.w > 4 && bounds.h > 4) {
        props.onPlaceElement(gesture.createType, { x: bounds.x, y: bounds.y, w: bounds.w, h: bounds.h });
      } else if (gesture.createType === "text" || gesture.createType === "image") {
        props.onPlaceElement(gesture.createType, { x: gesture.startDoc.x, y: gesture.startDoc.y });
      } else {
        // a tiny drag on shape tools creates a default-sized box
        props.onPlaceElement(gesture.createType, { x: gesture.startDoc.x, y: gesture.startDoc.y });
      }
      props.onPlacingDone();
      return;
    }
    if (gesture.origin && liveOverride && selected) {
      const patch: Partial<DrawElement> = { ...liveOverride };
      // keep line bbox in sync with its points
      if (patch.points) {
        const xs = patch.points.map((p) => p.x);
        const ys = patch.points.map((p) => p.y);
        patch.x = Math.min(...xs);
        patch.y = Math.min(...ys);
        patch.w = Math.max(...xs) - Math.min(...xs);
        patch.h = Math.max(...ys) - Math.min(...ys);
      }
      props.onPatchElement(selected.id, patch);
    }
    setLiveOverride(null);
  };

  const onWheel = (event: React.WheelEvent<HTMLDivElement>) => {
    event.preventDefault();
    const container = containerRef.current;
    if (!container) return;
    const rect = container.getBoundingClientRect();
    const mouseX = event.clientX - rect.left;
    const mouseY = event.clientY - rect.top;
    const factor = event.deltaY < 0 ? 1.12 : 1 / 1.12;
    const nextZoom = Math.min(8, Math.max(0.05, zoom * factor));
    const scale = nextZoom / zoom;
    setPan({
      x: mouseX - (mouseX - pan.x) * scale,
      y: mouseY - (mouseY - pan.y) * scale,
    });
    setZoom(nextZoom);
  };

  /* ---------------- context menu ---------------- */

  const onStageContextMenu = (event: React.MouseEvent<HTMLDivElement>) => {
    if (event.button !== 2) return;
    const point = toDoc(event.clientX, event.clientY);
    // visible & interactive elements win over hidden/locked ones underneath
    const hit = placing ? null : elementAt(point) ?? elementAt(point, true);
    if (hit && props.elementCtx) {
      if (hit.id !== selectedId) props.onSelect(hit.id);
      openCtxMenu(event, props.elementCtx(hit.id));
      return;
    }
    const base = props.canvasCtx?.(point) ?? [];
    const viewItems: MenuItem[] = [
      ...(base.length > 0 ? [{ type: "sep" } as MenuItem] : []),
      { label: t("drawImage.ctxFitView"), icon: "Maximize2", onSelect: () => window.setTimeout(fit, 0) },
      {
        label: t("drawImage.ctxZoom100"),
        icon: "ZoomIn",
        hint: `${Math.round(zoom * 100)}%`,
        onSelect: () => window.setTimeout(() => zoomTo(1), 0),
      },
    ];
    openCtxMenu(event, [...base, ...viewItems]);
  };

  /* ---------------- keyboard (space to pan) ---------------- */

  useEffect(() => {
    const down = (event: KeyboardEvent) => {
      if (event.code === "Space" && !isTypingTarget(event.target)) setSpaceHeld(true);
    };
    const up = (event: KeyboardEvent) => {
      if (event.code === "Space") setSpaceHeld(false);
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
    };
  }, []);

  /* ---------------- handle positions ---------------- */

  const handlePoints = useMemo(() => {
    if (!selected) return null;
    const bounds = elementBounds(selected);
    const cx = bounds.x + bounds.w / 2;
    const cy = bounds.y + bounds.h / 2;
    if (selected.type === "line") {
      return {
        rotate: null as null | { x: number; y: number },
        corners: selected.points.map((point) => rotatePoint(point.x, point.y, cx, cy, selected.rotation)),
        isLine: true,
      };
    }
    const local: Record<Handle, { x: number; y: number }> = {
      nw: { x: bounds.x, y: bounds.y },
      n: { x: cx, y: bounds.y },
      ne: { x: bounds.x + bounds.w, y: bounds.y },
      e: { x: bounds.x + bounds.w, y: cy },
      se: { x: bounds.x + bounds.w, y: bounds.y + bounds.h },
      s: { x: cx, y: bounds.y + bounds.h },
      sw: { x: bounds.x, y: bounds.y + bounds.h },
      w: { x: bounds.x, y: cy },
    };
    const rotated: Record<string, { x: number; y: number }> = {};
    for (const [key, point] of Object.entries(local)) {
      rotated[key] = rotatePoint(point.x, point.y, cx, cy, selected.rotation);
    }
    return {
      rotate: { x: cx, y: bounds.y - 22 / zoom },
      corners: rotated,
      isLine: false,
    };
  }, [selected, zoom]);

  /* ---------------- render ---------------- */

  const liveElement = selected && liveOverride ? ({ ...selected, ...liveOverride } as DrawElement) : null;
  const selectionBounds = liveElement ? elementBounds(liveElement) : null;

  return (
    <div
      ref={containerRef}
      className="relative h-full w-full overflow-hidden bg-ink-950"
      style={{ cursor: spaceHeld ? "grab" : placing ? "crosshair" : "default" }}
      onPointerDown={onStagePointerDown}
      onPointerMove={onStagePointerMove}
      onPointerUp={onStagePointerUp}
      onPointerLeave={() => props.onStatus?.({ zoom, cursor: null })}
      onWheel={onWheel}
      onContextMenu={onStageContextMenu}
    >
      <div className="absolute" style={{ left: pan.x, top: pan.y, width: doc.width * zoom, height: doc.height * zoom }}>
        {/* checkerboard + canvas */}
        <div className="absolute inset-0 checkerboard rounded-[2px] shadow-[0_0_0_1px_rgba(255,255,255,0.08)]" />
        <canvas ref={canvasRef} className="absolute left-0 top-0" />
        {props.showGrid && zoom > 0.35 ? (
          <div
            className="pointer-events-none absolute inset-0"
            style={{
              backgroundImage:
                "linear-gradient(to right, rgba(255,255,255,0.06) 1px, transparent 1px), linear-gradient(to bottom, rgba(255,255,255,0.06) 1px, transparent 1px)",
              backgroundSize: `${10 * zoom}px ${10 * zoom}px`,
            }}
          />
        ) : null}

        {/* hidden elements outline */}
        {doc.elements
          .filter((element) => !element.visible)
          .map((element) => {
            const bounds = elementBounds(element);
            return (
              <div
                key={`ghost-${element.id}`}
                className="pointer-events-none absolute border border-dashed border-ink-500/60"
                style={{ left: bounds.x * zoom, top: bounds.y * zoom, width: bounds.w * zoom, height: bounds.h * zoom }}
              />
            );
          })}

        {/* selection outline */}
        {selected && selectionBounds && !placing ? (
          <div
            className="pointer-events-none absolute border border-info/80"
            style={{
              left: selectionBounds.x * zoom,
              top: selectionBounds.y * zoom,
              width: selectionBounds.w * zoom,
              height: selectionBounds.h * zoom,
            }}
          />
        ) : null}

        {/* create preview */}
        {placing && liveOverride ? (
          <div
            className="pointer-events-none absolute border border-dashed border-info/80 bg-info/10"
            style={{
              left: (liveOverride.x ?? 0) * zoom,
              top: (liveOverride.y ?? 0) * zoom,
              width: (liveOverride.w ?? 0) * zoom,
              height: (liveOverride.h ?? 0) * zoom,
            }}
          />
        ) : null}
      </div>

      {/* handles rendered in screen space */}
      {selected && handlePoints && !placing ? (
        <div className="pointer-events-none absolute inset-0">
          {/* rotate handle */}
          {handlePoints.rotate ? (
            <ScreenHandle
              className="pointer-events-auto"
              point={handlePoints.rotate}
              pan={pan}
              zoom={zoom}
              onPointerDown={(event) => {
                event.stopPropagation();
                props.onBeginHistory();
                beginGesture({ kind: "rotate", startDoc: toDoc(event.clientX, event.clientY), origin: { ...selected } });
                (event.target as HTMLElement).setPointerCapture(event.pointerId);
              }}
              title={t("drawImage.rotate")}
            >
              <span className="text-[9px] leading-none">⟳</span>
            </ScreenHandle>
          ) : null}
          {/* resize handles or line endpoints */}
          {handlePoints.isLine
            ? (handlePoints.corners as { x: number; y: number }[]).map((point, index) => (
                <ScreenHandle
                  key={`ep-${index}`}
                  className="pointer-events-auto rounded-full"
                  point={point}
                  pan={pan}
                  zoom={zoom}
                  onPointerDown={(event) => {
                    event.stopPropagation();
                    props.onBeginHistory();
                    beginGesture({ kind: "endpoint", endpoint: index, startDoc: toDoc(event.clientX, event.clientY), origin: { ...selected } });
                    (event.target as HTMLElement).setPointerCapture(event.pointerId);
                  }}
                  title={t("drawImage.endpoint")}
                />
              ))
            : Object.entries(handlePoints.corners).map(([key, point]) => (
                <ScreenHandle
                  key={key}
                  className="pointer-events-auto"
                  point={point}
                  pan={pan}
                  zoom={zoom}
                  onPointerDown={(event) => {
                    event.stopPropagation();
                    props.onBeginHistory();
                    beginGesture({
                      kind: "resize",
                      handle: key as Handle,
                      startDoc: toDoc(event.clientX, event.clientY),
                      origin: { ...selected },
                    });
                    (event.target as HTMLElement).setPointerCapture(event.pointerId);
                  }}
                  title={key}
                />
              ))}
        </div>
      ) : null}

      {/* placing hint */}
      {placing ? (
        <div className="pointer-events-none absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full border border-ink-650 bg-ink-900/95 px-3 py-1.5 text-[11px] text-fg-subtle shadow-lg">
          {t("drawImage.placingHint", { type: t(`drawImage.types.${placing}`) })}
        </div>
      ) : null}
    </div>
  );
}

function snapBox(x: number, y: number, w: number, h: number, doc: DrawImageDoc): { x: number; y: number } {
  const targetsX = [0, doc.width / 2, doc.width];
  const targetsY = [0, doc.height / 2, doc.height];
  let bestX = x;
  let bestY = y;
  let bestDist = SNAP_DISTANCE;
  for (const target of targetsX) {
    for (const [edge, offset] of [
      [x, 0],
      [x + w / 2, w / 2],
      [x + w, w],
    ] as const) {
      const dist = Math.abs(edge - target);
      if (dist < bestDist) {
        bestDist = dist;
        bestX = target - offset;
      }
    }
  }
  bestDist = SNAP_DISTANCE;
  for (const target of targetsY) {
    for (const [edge, offset] of [
      [y, 0],
      [y + h / 2, h / 2],
      [y + h, h],
    ] as const) {
      const dist = Math.abs(edge - target);
      if (dist < bestDist) {
        bestDist = dist;
        bestY = target - offset;
      }
    }
  }
  return { x: bestX, y: bestY };
}

function ScreenHandle({
  point,
  pan,
  zoom,
  onPointerDown,
  className,
  title,
  children,
}: {
  point: { x: number; y: number };
  pan: { x: number; y: number };
  zoom: number;
  onPointerDown: (event: React.PointerEvent<HTMLDivElement>) => void;
  className?: string;
  title?: string;
  children?: React.ReactNode;
}) {
  return (
    <div
      title={title}
      onPointerDown={onPointerDown}
      className={`absolute grid place-items-center rounded-[2px] border border-ink-400 bg-ink-50 text-fg-onEmphasis ${className ?? ""}`}
      style={{
        left: pan.x + point.x * zoom - HANDLE_SIZE / 2,
        top: pan.y + point.y * zoom - HANDLE_SIZE / 2,
        width: HANDLE_SIZE,
        height: HANDLE_SIZE,
      }}
    >
      {children}
    </div>
  );
}

function isTypingTarget(target: EventTarget | null): boolean {
  if (!target || !(target instanceof HTMLElement)) return false;
  return target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable;
}
