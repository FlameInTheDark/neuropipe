import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { Edge, EditorComment, GraphNode, LibraryCategory, NodeGroup, PinDataType, PortKind } from "../types";
import { BODY_BOTTOM, BODY_TOP, HEADER_H, NODE_BORDER, NODE_W, ROW_H, nodeHeight, portX, portY } from "../data/graph";

/* footprint of a freshly-created library node (2 input + 2 output rows) */
const DROP_GHOST_H = NODE_BORDER * 2 + HEADER_H + BODY_TOP + 2 * ROW_H + BODY_BOTTOM;
import { NodeCard } from "./NodeCard";
import { RerouteNode, REROUTE_SIZE } from "./RerouteNode";
import { GroupFrame, type ResizeHandle } from "./GroupFrame";
import { CommentCard } from "./CommentCard";
import { nodesInGroup } from "@/features/graph/graph-ops";
import { Icon } from "./icons";
import { cn } from "../utils/cn";
import { useCtxMenu, type MenuItem } from "./ContextMenu";
import { Tooltip } from "./Tooltip";
import { NodePicker, type PickerAnchor } from "./NodePicker";
import { edgeColor, pinPalette } from "../lib/pins";
import { isReroute } from "@/features/graph/graph-ops";
import type { LibraryItem } from "../types";

export interface View {
  x: number;
  y: number;
  z: number;
}

type Pending = { node: string; port: string; kind: PortKind; dir: "in" | "out"; x: number; y: number };
type NodeDrag = {
  id: string;
  start: { x: number; y: number };
  origins: Record<string, { x: number; y: number }>;
};
type GroupDrag = {
  id: string;
  start: { x: number; y: number };
  origin: { x: number; y: number };
  members: Record<string, { x: number; y: number }>;
};
type GroupResize = {
  id: string;
  handle: ResizeHandle;
  start: { x: number; y: number };
  rect: { x: number; y: number; w: number; h: number };
};

function portPos(nodes: GraphNode[], nodeId: string, portId: string, dir: "in" | "out") {
  const n = nodes.find((x) => x.id === nodeId);
  if (!n) return null;
  // a reroute has both pins at its centre, so wires pass straight through it
  if (isReroute(n)) {
    return { x: n.x + REROUTE_SIZE / 2, y: n.y + REROUTE_SIZE / 2 };
  }
  const list = dir === "in" ? n.inputs : n.outputs;
  const i = list.findIndex((p) => p.id === portId);
  if (i < 0) return null;
  return { x: n.x + portX(dir), y: n.y + portY(i) };
}

function bezier(x1: number, y1: number, x2: number, y2: number) {
  const d = Math.max(36, Math.min(Math.abs(x2 - x1) * 0.6, 170));
  return `M ${x1} ${y1} C ${x1 + d} ${y1}, ${x2 - d} ${y2}, ${x2} ${y2}`;
}

export function Canvas({
  nodes,
  edges,
  library,
  selectedId,
  selectedIds,
  groups,
  selectedGroupId,
  renamingGroupId,
  view,
  snap,
  onSelect,
  onMove,
  onSelectGroup,
  onRenameGroup,
  onRenameGroupDone,
  onMoveGroup,
  onResizeGroup,
  onMoveMany,
  onToggleSelect,
  onClearSelection,
  selectMarquee,
  comments,
  selectedCommentId,
  setSelectedCommentId,
  renamingCommentId,
  setRenamingCommentId,
  onAddComment,
  onRenameComment,
  onResizeComment,
  onMoveComment,
  commentCtx,
  onConnect,
  onRemoveEdge,
  setView,
  setSnap,
  onDuplicate,
  onDelete,
  registerFit,
  leftInset = 12,
  rightInset = 12,
  nodeCtx,
  multiCtx,
  groupCtx,
  edgeCtx,
  portCtx,
  onAddNode,
}: {
  nodeCtx?: (id: string) => MenuItem[];
  /** bulk actions shown when right-clicking inside a multi-selection */
  multiCtx?: () => MenuItem[];
  groupCtx?: (id: string) => MenuItem[];
  /** receives the graph-space point of the right-click for wire splicing */
  edgeCtx?: (id: string, at: { x: number; y: number }) => MenuItem[];
  portCtx?: (nodeId: string, portId: string) => MenuItem[];
  onAddNode?: (item: LibraryItem, group: string, at: { x: number; y: number }) => void;
  leftInset?: number;
  rightInset?: number;
  registerFit?: (fn: () => void) => void;
  library?: LibraryCategory[];
  nodes: GraphNode[];
  edges: Edge[];
  groups: NodeGroup[];
  selectedId: string | null;
  selectedIds: Set<string>;
  selectedGroupId: string | null;
  renamingGroupId: string | null;
  view: View;
  snap: boolean;
  onSelect: (id: string | null) => void;
  onMove: (id: string, x: number, y: number) => void;
  onSelectGroup?: (id: string | null) => void;
  onRenameGroup?: (id: string, title: string) => void;
  /** clears the frame's own rename flag after inline editing finishes */
  onRenameGroupDone?: () => void;
  onMoveGroup?: (
    id: string,
    x: number,
    y: number,
    memberPositions: Record<string, { x: number; y: number }>,
  ) => void;
  onResizeGroup?: (id: string, rect: { x: number; y: number; w: number; h: number }) => void;
  /** batched positions for the whole dragged selection */
  onMoveMany?: (positions: Record<string, { x: number; y: number }>) => void;
  onToggleSelect?: (id: string) => void;
  onClearSelection?: () => void;
  selectMarquee?: (
    rect: { x: number; y: number; w: number; h: number },
    mode: "replace" | "add" | "subtract",
  ) => void;
  comments: EditorComment[];
  selectedCommentId: string | null;
  setSelectedCommentId: (id: string | null) => void;
  renamingCommentId: string | null;
  onAddComment?: (at: { x: number; y: number }) => void;
  onRenameComment?: (id: string, text: string) => void;
  onSetCommentColor?: (id: string, color: NodeGroup["color"]) => void;
  onResizeComment?: (id: string, rect: { x: number; y: number; w: number; h: number }) => void;
  onMoveComment?: (id: string, x: number, y: number) => void;
  onRemoveComment?: (id: string) => void;
  setRenamingCommentId: (id: string | null) => void;
  commentCtx?: (id: string) => MenuItem[];
  onConnect: (from: { node: string; port: string }, to: { node: string; port: string }, kind: PortKind) => void;
  onRemoveEdge: (id: string) => void;
  setView: (v: View | ((v: View) => View)) => void;
  setSnap: (v: boolean) => void;
  onDuplicate: () => void;
  onDelete: () => void;
}) {
  const wrap = useRef<HTMLDivElement>(null);
  const { t } = useTranslation();
  const [drag, setDrag] = useState<NodeDrag | null>(null);
  const [pan, setPan] = useState<{ cx: number; cy: number; vx: number; vy: number } | null>(null);
  const panMoved = useRef(false);
  const [pending, setPending] = useState<Pending | null>(null);
  const [hoverEdge, setHoverEdge] = useState<string | null>(null);
  const [picker, setPicker] = useState<PickerAnchor | null>(null);
  const openCtxMenu = useCtxMenu();
  const [legendOpen, setLegendOpen] = useState(false);
  const [dropAt, setDropAt] = useState<{ x: number; y: number } | null>(null);
  const [marqueeStart, setMarqueeStart] = useState<{ x: number; y: number } | null>(null);
  const [marquee, setMarquee] = useState<{ x: number; y: number; w: number; h: number } | null>(null);
  const [marqueeMode, setMarqueeMode] = useState<"replace" | "add" | "subtract">("replace");
  const [groupDrag, setGroupDrag] = useState<GroupDrag | null>(null);
  const [groupResize, setGroupResize] = useState<GroupResize | null>(null);
  const [commentDrag, setCommentDrag] = useState<{ id: string; start: { x: number; y: number }; orig: { x: number; y: number } } | null>(null);
  const [commentResize, setCommentResize] = useState<{ id: string; start: { x: number; y: number }; rect: { x: number; y: number; w: number; h: number } } | null>(null);

  const toGraph = useCallback(
    (cx: number, cy: number) => {
      const r = wrap.current!.getBoundingClientRect();
      return { x: (cx - r.left - view.x) / view.z, y: (cy - r.top - view.y) / view.z };
    },
    [view],
  );

  /* Edge colours follow the *live* source-port contract. Persisted edges do
     not store a data type — it is re-derived on load, and backend-resolved
     nodes (function calls, SQL, dynamic pins) only receive their real ports
     after an async resolve, so reading the wire's stored type would render
     stale gray "any" connections. Wires entering a reroute knot chain are
     traced back to the ultimate non-reroute source. */
  const nodeById = useMemo(() => {
    const map = new Map<string, GraphNode>();
    for (const n of nodes) map.set(n.id, n);
    return map;
  }, [nodes]);
  const incomingEdgeByNode = useMemo(() => {
    const map = new Map<string, Edge>();
    for (const e of edges) {
      if (!map.has(e.to.node)) map.set(e.to.node, e);
    }
    return map;
  }, [edges]);
  const edgeDataType = useCallback(
    (e: Edge): PinDataType | undefined => {
      if (e.kind === "exec" || e.kind === "tool") return e.kind;
      let cur = e;
      for (let guard = 0; guard < 64; guard++) {
        const src = nodeById.get(cur.from.node);
        if (!src) return cur.dataType ?? "any";
        if (!isReroute(src)) {
          const port = src.outputs.find((p) => p.id === cur.from.port);
          return port?.dataType ?? cur.dataType ?? "any";
        }
        const feed = incomingEdgeByNode.get(src.id);
        if (!feed) return cur.dataType ?? "any"; // dangling knot keeps its own type
        cur = feed;
      }
      return cur.dataType ?? "any";
    },
    [nodeById, incomingEdgeByNode],
  );

  /* single source of truth for where a dragged library node lands.
     both the preview ghost and the actual drop use this, so they can never diverge. */
  const dropPos = useCallback(
    (cx: number, cy: number) => {
      const g = toGraph(cx, cy);
      const snap8 = (v: number) => Math.round(v / 8) * 8;
      // anchor the card so the cursor sits on its header, not its corner
      return { x: snap8(g.x - NODE_W / 2), y: snap8(g.y - HEADER_H / 2) };
    },
    [toGraph],
  );

  const fit = useCallback(() => {
    const el = wrap.current;
    if (!el || nodes.length === 0) return;
    const minX = Math.min(...nodes.map((n) => n.x));
    const minY = Math.min(...nodes.map((n) => n.y));
    const maxX = Math.max(...nodes.map((n) => n.x + (isReroute(n) ? REROUTE_SIZE : NODE_W)));
    const maxY = Math.max(...nodes.map((n) => n.y + (isReroute(n) ? REROUTE_SIZE : nodeHeight(n))));
    const pad = 40;
    const availW = el.clientWidth - leftInset - rightInset - pad * 2;
    const availH = el.clientHeight - pad * 2;
    const z = Math.max(0.3, Math.min(1.1, Math.min(availW / Math.max(1, maxX - minX), availH / Math.max(1, maxY - minY))));
    setView({
      z,
      x: leftInset + pad + (availW - (maxX - minX) * z) / 2 - minX * z,
      y: pad + (availH - (maxY - minY) * z) / 2 - minY * z,
    });
  }, [nodes, setView, leftInset, rightInset]);

  useEffect(() => {
    const t = requestAnimationFrame(fit);
    return () => cancelAnimationFrame(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    registerFit?.(fit);
  }, [fit, registerFit]);

  /* wheel: always zoom at cursor (shift+scroll = pan) */
  useEffect(() => {
    const el = wrap.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const r = el.getBoundingClientRect();
      if (e.shiftKey) {
        // shift+scroll = horizontal/vertical pan
        setView((v) => ({ ...v, x: v.x - e.deltaX - e.deltaY, y: v.y }));
      } else {
        // normal scroll = zoom at cursor
        const px = e.clientX - r.left;
        const py = e.clientY - r.top;
        setView((v) => {
          const nz = Math.min(2.2, Math.max(0.25, v.z * Math.exp(-e.deltaY * 0.004)));
          return { z: nz, x: px - ((px - v.x) / v.z) * nz, y: py - ((py - v.y) / v.z) * nz };
        });
      }
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, [setView]);

  /* group frames: header drag moves the frame + its captured nodes, grips resize it */
  useEffect(() => {
    if (!groupDrag && !groupResize) return;

    const move = (e: PointerEvent) => {
      const p = toGraph(e.clientX, e.clientY);

      if (groupDrag) {
        const dx = p.x - groupDrag.start.x;
        const dy = p.y - groupDrag.start.y;
        const members = Object.fromEntries(
          Object.entries(groupDrag.members).map(([id, o]) => [id, { x: o.x + dx, y: o.y + dy }]),
        );
        onMoveGroup?.(groupDrag.id, groupDrag.origin.x + dx, groupDrag.origin.y + dy, members);
        return;
      }

      if (groupResize) {
        const dx = p.x - groupResize.start.x;
        const dy = p.y - groupResize.start.y;
        const { handle, rect } = groupResize;
        const next = { ...rect };
        if (handle.includes("w")) { next.x = rect.x + dx; next.w = rect.w - dx; }
        if (handle.includes("e")) { next.w = rect.w + dx; }
        if (handle.includes("n")) { next.y = rect.y + dy; next.h = rect.h - dy; }
        if (handle.includes("s")) { next.h = rect.h + dy; }
        onResizeGroup?.(groupResize.id, next);
      }
    };

    const up = () => {
      setGroupDrag(null);
      setGroupResize(null);
    };

    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    return () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
    };
  }, [groupDrag, groupResize, toGraph, onMoveGroup, onResizeGroup]);

  /* sticky notes: header drag moves the card, corner grip resizes it */
  useEffect(() => {
    if (!commentDrag && !commentResize) return;
    const move = (e: PointerEvent) => {
      const p = toGraph(e.clientX, e.clientY);
      if (commentDrag) {
        const dx = p.x - commentDrag.start.x;
        const dy = p.y - commentDrag.start.y;
        onMoveComment?.(commentDrag.id, Math.round(commentDrag.orig.x + dx), Math.round(commentDrag.orig.y + dy));
      }
      if (commentResize) {
        onResizeComment?.(commentResize.id, {
          x: commentResize.rect.x, y: commentResize.rect.y,
          w: commentResize.rect.w + (p.x - commentResize.start.x),
          h: commentResize.rect.h + (p.y - commentResize.start.y),
        });
      }
    };
    const up = () => { setCommentDrag(null); setCommentResize(null); };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    return () => { window.removeEventListener("pointermove", move); window.removeEventListener("pointerup", up); };
  }, [commentDrag, commentResize, toGraph, onMoveComment, onResizeComment]);

  /* global pointer handling for drag / pan / connect */
  useEffect(() => {
    if (!drag && !pan && !pending) return;
    const move = (e: PointerEvent) => {
      if (drag) {
        const p = toGraph(e.clientX, e.clientY);
        const rawDx = p.x - drag.start.x;
        const rawDy = p.y - drag.start.y;
        const primary = drag.origins[drag.id];
        const targetX = snap ? Math.round((primary.x + rawDx) / 8) * 8 : Math.round(primary.x + rawDx);
        const targetY = snap ? Math.round((primary.y + rawDy) / 8) * 8 : Math.round(primary.y + rawDy);
        const dx = targetX - primary.x;
        const dy = targetY - primary.y;
        const positions = Object.fromEntries(
          Object.entries(drag.origins).map(([id, origin]) => [id, { x: origin.x + dx, y: origin.y + dy }]),
        );
        // single node drags go through the same batched path
        if (Object.keys(positions).length === 1 && drag.id in positions) {
          onMove(drag.id, positions[drag.id].x, positions[drag.id].y);
        } else {
          onMoveMany?.(positions);
        }
      } else if (pan) {
        if (Math.abs(e.clientX - pan.cx) + Math.abs(e.clientY - pan.cy) > 3) panMoved.current = true;
        setView((v) => ({ ...v, x: pan.vx + (e.clientX - pan.cx), y: pan.vy + (e.clientY - pan.cy) }));
      } else if (pending) {
        const p = toGraph(e.clientX, e.clientY);
        setPending({ ...pending, x: p.x, y: p.y });
      }
    };
    const up = (e: PointerEvent) => {
      if (pan && !panMoved.current) {
        onClearSelection?.();
        onSelectGroup?.(null);
      }
      if (pending) {
        const el = document.elementFromPoint(e.clientX, e.clientY) as HTMLElement | null;
        const target = el?.closest("[data-port-node]") as HTMLElement | null;
        if (target) {
          const dir = target.dataset.portDir as "in" | "out";
          const node = target.dataset.portNode!;
          const port = target.dataset.portId!;
          if (dir !== pending.dir && node !== pending.node) {
            const from = pending.dir === "out" ? { node: pending.node, port: pending.port } : { node, port };
            const to = pending.dir === "out" ? { node, port } : { node: pending.node, port: pending.port };
            onConnect(from, to, pending.kind);
          }
        }
      }
      setDrag(null);
      setPan(null);
      setPending(null);
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    return () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
    };
  }, [drag, pan, pending, toGraph, onMove, onMoveMany, onConnect, onSelect, onClearSelection, onSelectGroup, setView, snap]);

  const onNodePointerDown = (e: React.PointerEvent, id: string) => {
    const portEl = (e.target as HTMLElement).closest("[data-port-node]") as HTMLElement | null;
    e.stopPropagation();
    const p = toGraph(e.clientX, e.clientY);
    if (portEl) {
      e.preventDefault();
      setPending({
        node: portEl.dataset.portNode!,
        port: portEl.dataset.portId!,
        kind: (portEl.dataset.portKind as PortKind) ?? "data",
        dir: portEl.dataset.portDir as "in" | "out",
        x: p.x,
        y: p.y,
      });
      return;
    }
    // Ctrl/Cmd toggles one node without disturbing the rest
    if (e.ctrlKey || e.metaKey) {
      e.preventDefault();
      onToggleSelect?.(id);
      return;
    }

    const dragIds = selectedIds.has(id) ? Array.from(selectedIds) : [id];
    if (!selectedIds.has(id)) onSelect(id);
    const origins = Object.fromEntries(
      nodes.filter((n) => dragIds.includes(n.id)).map((n) => [n.id, { x: n.x, y: n.y }]),
    );
    setDrag({ id, start: p, origins });
  };

  const connected = new Set<string>();
  edges.forEach((e) => {
    connected.add(`${e.from.node}:${e.from.port}:out`);
    connected.add(`${e.to.node}:${e.to.port}:in`);
  });

  const zoomBy = (f: number) => {
    const el = wrap.current!;
    const px = el.clientWidth / 2;
    const py = el.clientHeight / 2;
    setView((v) => {
      const nz = Math.min(2.2, Math.max(0.25, v.z * f));
      return { z: nz, x: px - ((px - v.x) / v.z) * nz, y: py - ((py - v.y) / v.z) * nz };
    });
  };

  return (
    <div
      ref={wrap}
      onContextMenu={(e) => {
        e.preventDefault();
        if ((e.target as HTMLElement).closest("[data-node]") || (e.target as HTMLElement).closest("[data-edge]")) return;
        const g = toGraph(e.clientX, e.clientY);
        setPicker({ x: e.clientX, y: e.clientY, gx: g.x, gy: g.y });
      }}
      onPointerDown={(e) => {
        if (e.button !== 0 && e.button !== 1) return;
        panMoved.current = false;
        const g = toGraph(e.clientX, e.clientY);
        // shift = marquee select
        if (e.button === 0 && e.shiftKey) {
          e.currentTarget.setPointerCapture(e.pointerId);
          // alt subtracts from the selection, ctrl/cmd adds to it
          setMarqueeMode(e.altKey ? "subtract" : e.ctrlKey || e.metaKey ? "add" : "replace");
          setMarqueeStart(g);
          setMarquee({ x: g.x, y: g.y, w: 0, h: 0 });
          return;
        }
        setMarqueeStart(null);
        setPan({ cx: e.clientX, cy: e.clientY, vx: view.x, vy: view.y });
      }}
      onPointerMove={(e) => {
        if (!marqueeStart) return;
        const g = toGraph(e.clientX, e.clientY);
        setMarquee({ x: marqueeStart.x, y: marqueeStart.y, w: g.x - marqueeStart.x, h: g.y - marqueeStart.y });
      }}
      onPointerUp={(e) => {
        if (marqueeStart) {
          const g = toGraph(e.clientX, e.clientY);
          const rect = { x: marqueeStart.x, y: marqueeStart.y, w: g.x - marqueeStart.x, h: g.y - marqueeStart.y };
          selectMarquee?.(rect, marqueeMode);
          if (e.currentTarget.hasPointerCapture(e.pointerId)) {
            e.currentTarget.releasePointerCapture(e.pointerId);
          }
          setMarqueeStart(null);
          setMarquee(null);
          return;
        }
        setPan(null);
      }}
      onPointerCancel={() => {
        setMarqueeStart(null);
        setMarquee(null);
        setPan(null);
      }}
      onDragOver={(e) => {
        if (!e.dataTransfer.types.includes("application/x-neuropipe-node")) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = "copy";
        setDropAt(dropPos(e.clientX, e.clientY));
      }}
      onDragLeave={(e) => {
        // ignore transitions between children
        if (e.currentTarget.contains(e.relatedTarget as Node)) return;
        setDropAt(null);
      }}
      onDrop={(e) => {
        const raw = e.dataTransfer.getData("application/x-neuropipe-node");
        setDropAt(null);
        if (!raw) return;
        e.preventDefault();
        try {
          const { item, category } = JSON.parse(raw) as { item: LibraryItem; category: string };
          // identical anchor + snapping as the preview ghost
          onAddNode?.(item, category, dropPos(e.clientX, e.clientY));
        } catch {
          /* malformed payload — ignore */
        }
      }}
      className={cn(
        "relative h-full w-full min-h-0 flex-1 overflow-hidden bg-ink-950 select-none",
        pan ? "cursor-grabbing" : "cursor-default",
      )}
      style={{
        backgroundImage: "radial-gradient(circle, var(--canvas-grid) 1px, transparent 1px)",
        backgroundSize: `${26 * view.z}px ${26 * view.z}px`,
        backgroundPosition: `${view.x}px ${view.y}px`,
      }}
    >
      {/* vignette */}
      <div
        className="pointer-events-none absolute inset-0"
        style={{ background: "radial-gradient(ellipse at center, transparent 35%, var(--canvas-vignette))" }}
      />

      <div
        className="absolute top-0 left-0 origin-top-left"
        style={{ transform: `translate3d(${view.x}px, ${view.y}px, 0) scale(${view.z})` }}
      >
        <svg className="pointer-events-none absolute overflow-visible" width="1" height="1">
          {edges.map((e) => {
            const a = portPos(nodes, e.from.node, e.from.port, "out");
            const b = portPos(nodes, e.to.node, e.to.port, "in");
            if (!a || !b) return null;
            const active = selectedId === e.from.node || selectedId === e.to.node;
            const hover = hoverEdge === e.id;
            const dataType = edgeDataType(e);
            const d = bezier(a.x, a.y, b.x, b.y);
            return (
              <g key={e.id} data-edge={e.id} className="pointer-events-auto">
                <path
                  d={d}
                  fill="none"
                  stroke="transparent"
                  strokeWidth={12}
                  className="cursor-pointer"
                  onPointerEnter={() => setHoverEdge(e.id)}
                  onPointerLeave={() => setHoverEdge(null)}
                  onContextMenu={(ev) => {
                    ev.preventDefault();
                    ev.stopPropagation();
                    edgeCtx && openCtxMenu(ev, edgeCtx(e.id, toGraph(ev.clientX, ev.clientY)));
                  }}
                  onPointerDown={(ev) => ev.stopPropagation()}
                  onClick={(ev) => {
                    ev.stopPropagation();
                    onRemoveEdge(e.id);
                  }}
                />
                <path
                  d={d}
                  fill="none"
                  strokeLinecap="round"
                  strokeWidth={hover ? 2.2 : active ? 1.8 : 1.3}
                  className={cn(e.kind === "exec" && "edge-dash")}
                  style={{
                    transition: "stroke 0.15s",
                    stroke: edgeColor(e.kind, dataType, active, hover),
                  }}
                />
                {e.kind === "data" && (
                  <circle
                    cx={b.x}
                    cy={b.y}
                    r={2.5}
                    opacity={active ? 1 : 0.7}
                    style={{ fill: pinPalette(dataType).dot }}
                  />
                )}
              </g>
            );
          })}

          {pending && (() => {
            const srcNode = nodes.find((n) => n.id === pending.node);
            const srcPort = srcNode ? (pending.dir === "out" ? srcNode.outputs : srcNode.inputs).find((p) => p.id === pending.port) : undefined;
            const dragColor = srcPort ? pinPalette(srcPort.dataType).dot : "var(--fg)";
            return (
              <path
                d={
                  pending.dir === "out"
                    ? bezier(
                        portPos(nodes, pending.node, pending.port, "out")?.x ?? pending.x,
                        portPos(nodes, pending.node, pending.port, "out")?.y ?? pending.y,
                        pending.x,
                        pending.y,
                      )
                    : bezier(
                        pending.x,
                        pending.y,
                        portPos(nodes, pending.node, pending.port, "in")?.x ?? pending.x,
                        portPos(nodes, pending.node, pending.port, "in")?.y ?? pending.y,
                      )
                }
                fill="none"
                strokeWidth={1.4}
                strokeDasharray="3 4"
                opacity={0.85}
                style={{ stroke: dragColor }}
              />
            );
          })()}
        </svg>

        {/* group frames sit behind the graph so wires and nodes stay readable */}
        {groups.map((g) => (
          <GroupFrame
            key={g.id}
            group={g}
            selected={selectedGroupId === g.id}
            memberCount={nodesInGroup(nodes, g).length}
            onSelect={() => onSelectGroup?.(g.id)}
            autoEdit={renamingGroupId === g.id}
            onEditDone={() => onRenameGroupDone?.()}
            onRename={(title) => onRenameGroup?.(g.id, title)}
            onContextMenu={(e) => {
              e.preventDefault();
              e.stopPropagation();
              onSelectGroup?.(g.id);
              groupCtx && openCtxMenu(e, groupCtx(g.id));
            }}
            onHeaderPointerDown={(e) => {
              e.stopPropagation();
              const start = toGraph(e.clientX, e.clientY);
              const members = Object.fromEntries(
                nodesInGroup(nodes, g).map((id) => {
                  const n = nodes.find((x) => x.id === id)!;
                  return [id, { x: n.x, y: n.y }];
                }),
              );
              setGroupDrag({ id: g.id, start, origin: { x: g.x, y: g.y }, members });
            }}
            onResizePointerDown={(e, handle) => {
              const start = toGraph(e.clientX, e.clientY);
              setGroupResize({ id: g.id, handle, start, rect: { x: g.x, y: g.y, w: g.w, h: g.h } });
            }}
          />
        ))}

        {/* sticky notes sit behind nodes but above group frames */}
        {comments.map((c) => (
          <CommentCard
            key={c.id}
            comment={c}
            selected={selectedCommentId === c.id}
            autoEdit={renamingCommentId === c.id}
            onEditDone={() => setRenamingCommentId?.(null) ?? undefined}
            onSelect={() => setSelectedCommentId?.(c.id)}
            onRename={(text) => onRenameComment?.(c.id, text)}
            onContextMenu={(e) => {
              e.preventDefault();
              e.stopPropagation();
              setSelectedCommentId?.(c.id);
              commentCtx && openCtxMenu(e, commentCtx(c.id));
            }}
            onHeaderPointerDown={(e) => {
              e.stopPropagation();
              const start = toGraph(e.clientX, e.clientY);
              setCommentDrag({ id: c.id, start, orig: { x: c.x, y: c.y } });
            }}
            onResizePointerDown={(e) => {
              e.stopPropagation();
              const start = toGraph(e.clientX, e.clientY);
              setCommentResize({ id: c.id, start, rect: { x: c.x, y: c.y, w: c.w, h: c.h } });
            }}
          />
        ))}

        {/* drop preview ghost — matches the real card footprint (2 pin rows) */}
        {dropAt && (
          <div
            className="pointer-events-none absolute rounded-[10px] border border-dashed border-ink-300/70 bg-ink-800/40"
            style={{ left: dropAt.x, top: dropAt.y, width: NODE_W, height: DROP_GHOST_H }}
          >
            <div
              style={{ height: HEADER_H }}
              className="flex items-center gap-1.5 border-b border-dashed border-ink-600/70 px-2.5 text-[11px] font-medium text-fg-muted"
            >
              <Icon name="Plus" className="h-3.5 w-3.5" />
              {t("canvas.dropToPlace")}
            </div>
          </div>
        )}

        {/* shared outline makes a multi-selection read as one editable group */}
        {(() => {
          const selNodes = nodes.filter((n) => selectedIds.has(n.id));
          if (selNodes.length <= 1) return null;
          const minX = Math.min(...selNodes.map((n) => n.x));
          const minY = Math.min(...selNodes.map((n) => n.y));
          const maxX = Math.max(...selNodes.map((n) => n.x + (isReroute(n) ? REROUTE_SIZE : NODE_W)));
          const maxY = Math.max(...selNodes.map((n) => n.y + (isReroute(n) ? REROUTE_SIZE : nodeHeight(n))));
          return (
            <div
              className="pointer-events-none absolute rounded-xl border border-ink-300/55 bg-ink-300/[0.025] shadow-[0_0_0_1px_rgba(236,237,241,0.035)_inset]"
              style={{
                left: minX - 10,
                top: minY - 10,
                width: maxX - minX + 20,
                height: maxY - minY + 20,
              }}
            >
              <span className="absolute -top-[24px] left-0 flex h-5 items-center gap-1.5 rounded-md border border-ink-600 bg-ink-800/95 px-2 text-[10px] font-medium text-fg-muted shadow-lg shadow-black/30">
                <Icon name="Boxes" className="h-3 w-3 text-fg-subtle" />
                {t("editor.nSelected", { count: selNodes.length })}
              </span>
            </div>
          );
        })()}

        {nodes.map((n) => {
          const shared = {
            node: n,
            selected: selectedIds.has(n.id),
            primary: selectedId === n.id,
            connectedPorts: connected,
            onPointerDown: onNodePointerDown,
            // Pointer-down owns selection so Ctrl/Cmd toggles are not undone by click.
            onSelect: () => {},
            onContextMenu: (e: React.MouseEvent) => {
              if ((e.target as HTMLElement).closest("[data-port-node]")) return; // ports get their own menu
              e.preventDefault();
              e.stopPropagation();
              // right-clicking inside a multi-selection keeps it and shows bulk actions
              if (selectedIds.has(n.id) && selectedIds.size > 1) {
                multiCtx && openCtxMenu(e, multiCtx());
                return;
              }
              if (!selectedIds.has(n.id)) onSelect(n.id);
              nodeCtx && openCtxMenu(e, nodeCtx(n.id));
            },
            onPortContextMenu: (e: React.MouseEvent, portId: string) => {
              e.preventDefault();
              e.stopPropagation();
              onSelect(n.id);
              portCtx && openCtxMenu(e, portCtx(n.id, portId));
            },
          };
          return isReroute(n) ? (
            <RerouteNode key={n.id} {...shared} />
          ) : (
            <NodeCard key={n.id} {...shared} />
          );
        })}
      </div>

      {/* flow chip + vertical legend */}
      <div
        style={{ left: leftInset }}
        className="absolute top-3 z-30 flex flex-col items-start transition-[left] duration-200"
      >
        <Tooltip content={t("canvas.toggleLegend")} side="bottom">
          <button
            onClick={() => setLegendOpen((v) => !v)}
            aria-expanded={legendOpen}
            aria-label={t("canvas.toggleLegend")}
            className="flex h-7 shrink-0 items-center gap-1 rounded-lg border border-ink-700 bg-ink-850/85 px-2 text-[10.5px] font-medium text-fg shadow-[0_10px_30px_-14px_rgba(0,0,0,0.9)] backdrop-blur transition hover:bg-ink-750"
          >
            <Icon name="LayoutGrid" className="h-3.5 w-3.5 text-fg-subtle" />
            {t("canvas.flow")}
            <Icon
              name="ChevronDown"
              className={cn("h-3 w-3 text-fg-faint transition-transform", !legendOpen && "-rotate-90")}
            />
          </button>
        </Tooltip>

        {legendOpen && (
          <div className="pop-in mt-1 flex flex-col gap-0.5 rounded-lg border border-ink-700 bg-ink-850/95 p-1 shadow-[0_10px_30px_-14px_rgba(0,0,0,0.9)] backdrop-blur">
            {(["exec", "text", "number", "boolean", "array", "map", "object", "bytes"] as const).map((lt) => {
              const c = pinPalette(lt);
              return (
                <Tooltip
                  key={lt}
                  content={`${c.name} — ${t(`pins.type_${lt}`)}`}
                  side="right"
                  className="w-full"
                >
                  <button
                    className="flex w-full shrink-0 items-center gap-1.5 whitespace-nowrap rounded px-1.5 py-1 text-[10px] text-fg-subtle transition hover:bg-ink-750 hover:text-fg"
                  >
                    <span className={lt === "exec" ? "h-[6px] w-[6px] rounded-[1px] shrink-0" : "h-[6px] w-[6px] rounded-full shrink-0"} style={{ background: c.dot }} />
                    {t(`pins.type_${lt}`)}
                  </button>
                </Tooltip>
              );
            })}
          </div>
        )}
      </div>

      {/* marquee overlay */}
      {marquee && (
        <div
          className="pointer-events-none absolute z-20 rounded-[3px] border border-ink-200/80 bg-ink-200/10 shadow-[0_0_0_1px_rgba(236,237,241,0.04)_inset]"
          style={{
            left: view.x + Math.min(marquee.x, marquee.x + marquee.w) * view.z,
            top: view.y + Math.min(marquee.y, marquee.y + marquee.h) * view.z,
            width: Math.abs(marquee.w) * view.z,
            height: Math.abs(marquee.h) * view.z,
          }}
        />
      )}

      {/* floating toolbar */}
      <div className="absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-0.5 rounded-lg border border-ink-700 bg-ink-850/90 p-1 shadow-[0_10px_30px_-12px_rgba(0,0,0,0.9)] backdrop-blur">
        <TB icon="ZoomOut" label={t("editor.zoomOut")} onClick={() => zoomBy(1 / 1.2)} />
        <Tooltip content={t("editor.resetZoom")} side="top">
          <button
            onClick={() => setView((v) => ({ ...v, z: 1 }))}
            aria-label={t("editor.resetZoom")}
            className="h-7 min-w-[46px] rounded-md px-1 font-mono text-[11px] text-fg-subtle transition hover:bg-ink-750 hover:text-fg"
          >
            {Math.round(view.z * 100)}%
          </button>
        </Tooltip>
        <TB icon="ZoomIn" label={t("editor.zoomIn")} onClick={() => zoomBy(1.2)} />
        <span className="mx-1 h-4 w-px bg-ink-700" />
        <TB icon="Maximize2" label={t("editor.fitGraph")} onClick={fit} />
        <TB icon="Magnet" label={t("editor.snapToGrid")} active={snap} onClick={() => setSnap(!snap)} />
        <span className="mx-1 h-4 w-px bg-ink-700" />
        {selectedIds.size > 1 && (
          <span className="flex h-7 items-center gap-1.5 rounded-md bg-ink-700/80 px-2 text-[10.5px] font-medium text-fg">
            <Icon name="Boxes" className="h-3 w-3 text-fg-subtle" />
            {selectedIds.size}
          </span>
        )}
        <TB icon="Copy" label={selectedIds.size > 1 ? t("editor.duplicateSelection") : t("editor.duplicateNode")} onClick={onDuplicate} disabled={!selectedId} />
        <TB icon="Trash2" label={selectedIds.size > 1 ? t("editor.deleteNodes", { count: selectedIds.size }) : t("editor.deleteNode")} onClick={onDelete} disabled={!selectedId} danger />
      </div>

      {/* minimap */}
      <Minimap nodes={nodes} selectedId={selectedId} selectedIds={selectedIds} rightInset={rightInset} />

      {/* node search palette (right-click on empty canvas) */}
      {picker && (
        <NodePicker
          at={picker}
          library={library ?? []}
          snap={snap}
          onFit={fit}
          onToggleSnap={() => setSnap(!snap)}
          onClose={() => setPicker(null)}
          onPick={(item, group) => {
            onAddNode?.(item, group, { x: picker.gx, y: picker.gy });
            setPicker(null);
          }}
          onAddComment={onAddComment}
        />
      )}
    </div>
  );
}

function TB({
  icon,
  label,
  onClick,
  active,
  disabled,
  danger,
}: {
  icon: string;
  label: string;
  onClick?: () => void;
  active?: boolean;
  disabled?: boolean;
  danger?: boolean;
}) {
  return (
    <Tooltip content={label} side="top">
      <button
        onClick={onClick}
        disabled={disabled}
        aria-label={label}
        className={cn(
          "grid h-7 w-7 place-items-center rounded-md text-fg-subtle transition",
          "hover:bg-ink-750 hover:text-fg active:scale-95",
          active && "bg-ink-700 text-fg",
          disabled && "cursor-not-allowed text-fg-faint hover:bg-transparent hover:text-fg-faint",
          danger && !disabled && "hover:bg-danger/15 hover:text-danger-fg",
        )}
      >
        <Icon name={icon} className="h-[15px] w-[15px]" />
      </button>
    </Tooltip>
  );
}

function Minimap({
  nodes,
  selectedId,
  selectedIds,
  rightInset = 12,
}: {
  nodes: GraphNode[];
  selectedId: string | null;
  selectedIds: Set<string>;
  rightInset?: number;
}) {
  const W = 158;
  const H = 92;
  if (nodes.length === 0) return null;
  const minX = Math.min(...nodes.map((n) => n.x)) - 30;
  const minY = Math.min(...nodes.map((n) => n.y)) - 30;
  const maxX = Math.max(...nodes.map((n) => n.x + (isReroute(n) ? REROUTE_SIZE : NODE_W))) + 30;
  const maxY = Math.max(...nodes.map((n) => n.y + (isReroute(n) ? REROUTE_SIZE : nodeHeight(n)))) + 30;
  const s = Math.min(W / (maxX - minX), H / (maxY - minY));

  return (
    <div
      style={{ right: rightInset }}
      className="absolute bottom-3 rounded-lg border border-ink-700 bg-ink-900/85 p-1.5 shadow-[0_10px_30px_-12px_rgba(0,0,0,0.9)] backdrop-blur transition-[right] duration-200"
    >
      <div className="relative" style={{ width: W, height: H }}>
        {nodes.map((n) => (
          <div
            key={n.id}
            className={cn(
              "absolute rounded-[2px]",
              selectedIds.has(n.id)
                ? selectedId === n.id
                  ? "bg-ink-50"
                  : "bg-ink-200/70"
                : n.status === "running"
                  ? "bg-ink-300/50"
                  : "bg-ink-600",
            )}
            style={{
              left: (n.x - minX) * s,
              top: (n.y - minY) * s,
              width: Math.max(3, (isReroute(n) ? REROUTE_SIZE : NODE_W) * s),
              height: Math.max(2, (isReroute(n) ? REROUTE_SIZE : nodeHeight(n)) * s),
            }}
          />
        ))}
      </div>
    </div>
  );
}





