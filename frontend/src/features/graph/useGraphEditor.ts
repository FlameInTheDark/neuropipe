import { useCallback, useEffect, useRef, useState } from "react";
import i18n from "@/i18n";
import { desktop } from "@/lib/bridge";
import { REROUTE_SIZE } from "@/components/RerouteNode";
import type {
  CustomFunction,
  Execution,
  NodeDefinition,
  Pipeline,
} from "@/lib/types";
import type { EditorComment, NodeGroup } from "@/types";
import type { Edge, FunctionKind, GraphNode, LibraryItem, LogEntry, Port, PortKind } from "@/types";
import {
  applyRunStatus,
  dehydrate,
  fnKindFromBackend,
  fnKindToBackend,
  functionPinsFromPorts,
  hydrateGraph,
  isBackendResolvedType,
  localizeDefinitions,
  mapDataType,
  nodeRunToLog,
  refreshNode,
  type DefinitionIndex,
} from "@/lib/adapters";
import type { NodePort } from "@/lib/types";
import { isTypeAssignable } from "@/lib/type-spec";
import {
  applyFunctionInterface,
  createGroupFromNodes,
  createCommentNode,
  normalizeCommentRect,
  nodeBox,
  nodesInGroup,
  normalizeGroupRect,
  applyFunctionKind,
  bridgeRerouteOnDelete,
  clearRerouteTypeIfOrphaned,
  createRerouteNode,
  detachInput,
  duplicateNode,
  edgeExists,
  isReroute,
  makeNodeId,
  patchNode,
  propagateRerouteType,
  removeEdgesForNode,
  removeEdgesForPort,
  resetStatuses,
  REROUTE_IN,
  REROUTE_OUT,
  setNodeValue,
} from "./graph-ops";

export interface GraphSnapshot {
  nodes: GraphNode[];
  edges: Edge[];
}

export interface Viewport {
  x: number;
  y: number;
  z: number;
}

const EMPTY: { nodes: GraphNode[]; edges: Edge[] } = { nodes: [], edges: [] };

/* ------------------------------------------------------------------ */
/*  Backend round-trip for reroutes                                    */
/* ------------------------------------------------------------------ */

/**
 * Reroute knots are presentation-only. The Blueprint V3 model has no reroute
 * nodes: on save every knot chain collapses into a single edge carrying the
 * knot positions as waypoints; on load such edges are expanded back into
 * knots. Legacy flow:/data:reroute nodes (V2 revisions) still open as knots.
 */

function isLegacyRerouteNode(type: string): boolean {
  return type === "flow:reroute" || type === "data:reroute";
}

/** Saved waypoint wires expand into pin-sized UI knots. */
function expandWaypoints(nodes: GraphNode[], edges: Edge[]): { nodes: GraphNode[]; edges: Edge[] } {
  const knots: GraphNode[] = [];
  const plain: Edge[] = [];
  for (const e of edges) {
    const wps = e.waypoints ?? [];
    if (!wps.length) {
      plain.push(e);
      continue;
    }
    let from = e.from;
    wps.forEach((wp, i) => {
      const id = `reroute-${crypto.randomUUID().slice(0, 8)}`;
      const dataType = i === 0 && e.kind === "exec" ? ("exec" as const) : (e.dataType ?? "any");
      knots.push(createRerouteNode(id, wp.x - REROUTE_SIZE / 2, wp.y - REROUTE_SIZE / 2, dataType as never));
      plain.push({
        id: `${e.id}-w${i}`,
        from,
        to: { node: id, port: REROUTE_IN },
        kind: e.kind,
        dataType: e.dataType,
      });
      from = { node: id, port: REROUTE_OUT };
    });
    plain.push({
      id: `${e.id}-end`,
      from,
      to: e.to,
      kind: e.kind,
      dataType: e.dataType,
    });
  }
  return { nodes: [...nodes, ...knots], edges: plain };
}

/** Legacy V2 reroute *nodes* also open as knots (validator rejects them in V3). */
function toUiReroutes(nodes: GraphNode[], edges: Edge[]): { nodes: GraphNode[]; edges: Edge[] } {
  if (!nodes.some((n) => isLegacyRerouteNode(n.type))) return { nodes, edges };
  let nextEdges = edges;
  const nextNodes = nodes.map((n) => {
    if (!isLegacyRerouteNode(n.type)) return n;
    const isExec = n.type === "flow:reroute";
    let dataType = isExec ? "exec" : "any";
    if (!isExec) {
      const fed = edges.find((e) => e.to.node === n.id) ?? edges.find((e) => e.from.node === n.id);
      if (fed?.dataType && fed.dataType !== "exec") dataType = fed.dataType;
      else if (fed && fed.kind === "exec") dataType = "exec";
    }
    nextEdges = nextEdges.map((e) => ({
      ...e,
      to: e.to.node === n.id ? { node: n.id, port: REROUTE_IN } : e.to,
      from: e.from.node === n.id ? { node: n.id, port: REROUTE_OUT } : e.from,
    }));
    return createRerouteNode(n.id, n.x, n.y, dataType as never);
  });
  return { nodes: nextNodes, edges: nextEdges };
}

interface CollapsedEdge extends Edge {
  waypoints?: { x: number; y: number }[];
}

/** Collapse every knot chain into one direct edge with waypoints. */
export function collapseReroutes(
  nodes: GraphNode[],
  edges: Edge[],
): { nodes: GraphNode[]; edges: CollapsedEdge[] } {
  const knotIds = new Set(nodes.filter(isReroute).map((n) => n.id));
  if (knotIds.size === 0) return { nodes, edges };
  const byId = new Map(nodes.map((n) => [n.id, n]));

  const outEdges: CollapsedEdge[] = [];
  const visited = new Set<string>();
  for (const e0 of edges) {
    // only start walks at chain heads entering a knot from a real node
    if (knotIds.has(e0.from.node) || !knotIds.has(e0.to.node)) continue;

    const waypoints: { x: number; y: number }[] = [];
    let cur: Edge | undefined = e0;
    let deadEnd = false;
    while (cur && knotIds.has(cur.to.node)) {
      visited.add(cur.id);
      const knot = byId.get(cur.to.node);
      if (!knot) { deadEnd = true; break; }
      waypoints.push({ x: knot.x + REROUTE_SIZE / 2, y: knot.y + REROUTE_SIZE / 2 });
      cur = edges.find((nx) => nx.from.node === cur!.to.node);
      if (!cur) { deadEnd = true; break; }
      visited.add(cur.id);
    }
    // a dangling chain tail (knot with no outgoing wire) drops the whole walk
    if (deadEnd || !cur) continue;

    outEdges.push({
      id: e0.id,
      from: e0.from,
      to: cur.to,
      kind: e0.kind,
      dataType: e0.dataType,
      ...(waypoints.length ? { waypoints } : {}),
    });
  }

  // keep untouched edges; drop every segment that belonged to a walked chain,
  // plus orphan knot stubs that never had an incoming wire
  for (const e of edges) {
    if (visited.has(e.id)) continue;
    if (knotIds.has(e.from.node)) continue;
    outEdges.push(e);
  }

  const keptNodes = nodes.filter((n) => !isReroute(n));
  return { nodes: keptNodes, edges: outEdges };
}

/** Data-type assignability for canvas connections. Prefers the full
 *  recursive contract when both pins carry one (mirrors Go typespec.Assignable),
 *  falling back to the coarse pin type. */
function typesCompatible(
  source: Port | undefined,
  target: Port | undefined,
): boolean {
  if (!source || !target) return false;
  if (source.kind !== target.kind) return false;
  if (source.kind === "exec") return true;
  if (source.spec && target.spec) return isTypeAssignable(source.spec, target.spec);

  const t = target.dataType ?? "any";
  if (t === "any") return true;
  const s = source.dataType ?? "any";
  if (s !== t) return false;
  if (s === "array") {
    const ta = target.arrayOf ?? "any";
    return ta === "any" || (source.arrayOf ?? "any") === ta;
  }
  return true;
}

/**
 * Owns the node graph of the currently open pipeline or function and every
 * mutation applied to it, including backend round-trips
 * (resolveNodeDefinition, run draft, execution history).
 */
export function useGraphEditor(options: {
  notify: (text: string, icon?: string) => void;
  definitionIndex: DefinitionIndex;
  runningMap: Record<string, boolean>;
}) {
  const { notify, definitionIndex, runningMap } = options;

  const [mode, setMode] = useState<"pipeline" | "function" | null>(null);
  const [pipeline, setPipeline] = useState<Pipeline | null>(null);
  const [fn, setFn] = useState<CustomFunction | null>(null);
  const [nodes, setNodes] = useState<GraphNode[]>(EMPTY.nodes);
  const [edges, setEdges] = useState<Edge[]>(EMPTY.edges);
  const [log, setLog] = useState<LogEntry[]>([]);
  const [executions, setExecutions] = useState<Execution[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [loadError, setLoadError] = useState(false);

  const clipboard = useRef<GraphNode | null>(null);
  const viewRef = useRef<Viewport>({ x: 40, y: 40, z: 1 });
  const seq = useRef(0);

  useEffect(() => () => void ++seq.current, []);

  const selected = nodes.find((n) => n.id === selectedId) ?? null;
  const touch = useCallback(() => setDirty(true), []);

  /* ---------- multi-selection ---------- */

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  /** UI-only grouping frames; never persisted with the pipeline. */
  const [groups, setGroups] = useState<NodeGroup[]>([]);
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null);
  /** Group whose title input should be focused for editing. */
  const [renamingGroupId, setRenamingGroupId] = useState<string | null>(null);

  const selectOnly = useCallback((id: string | null) => {
    setSelectedId(id);
    setSelectedIds(id ? new Set([id]) : new Set());
  }, []);

  /** Ctrl/Cmd-click toggles one node without disturbing the rest. */
  const toggleSelect = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      setSelectedId((primary) => {
        if (next.size === 0) return null;
        if (primary && next.has(primary)) return primary;
        return next.values().next().value ?? null;
      });
      return next;
    });
  }, []);

  const clearSelection = useCallback(() => {
    setSelectedIds(new Set());
    setSelectedId(null);
  }, []);

  const selectAll = useCallback(() => {
    const all = new Set(nodes.map((n) => n.id));
    setSelectedIds(all);
    setSelectedId(nodes[0]?.id ?? null);
  }, [nodes]);

  /** Marquee sweep. replace = fresh, add = union, subtract = remove swept. */
  const selectMarquee = useCallback(
    (rect: { x: number; y: number; w: number; h: number }, mode: "replace" | "add" | "subtract" = "replace") => {
      const minX = Math.min(rect.x, rect.x + rect.w);
      const maxX = Math.max(rect.x, rect.x + rect.w);
      const minY = Math.min(rect.y, rect.y + rect.w === undefined ? rect.y + rect.h : rect.y + rect.h);
      const maxY = Math.max(rect.y, rect.y + rect.h);
      const inside = nodes
        .filter((n) => {
          const box = nodeBox(n);
          // touching the marquee is enough; feels better for large nodes
          return box.x < maxX && box.x + box.w > minX && box.y < maxY && box.y + box.h > minY;
        })
        .map((n) => n.id);
      setSelectedIds((prev) => {
        const next = mode === "replace" ? new Set<string>() : new Set(prev);
        inside.forEach((id) => (mode === "subtract" ? next.delete(id) : next.add(id)));
        setSelectedId((primary) =>
          primary && next.has(primary) ? primary : (next.values().next().value ?? null),
        );
        return next;
      });
    },
    [nodes],
  );

  /** Apply absolute positions to a whole selection in one state update. */
  const moveNodes = useCallback((positions: Record<string, { x: number; y: number }>) => {
    setNodes((ns) => ns.map((n) => (positions[n.id] ? { ...n, ...positions[n.id] } : n)));
    touch();
  }, [touch]);

  /** Minimum breathing room kept between nodes pushed apart by an align. */
  const ALIGN_GAP = 24;

  const alignSelection = useCallback(
    (axis: "left" | "right" | "top" | "bottom" | "hcenter" | "vcenter") => {
      const ids = selectedIds.size ? Array.from(selectedIds) : selectedId ? [selectedId] : [];
      const sel = nodes.filter((n) => ids.includes(n.id));
      if (sel.length < 2) return;

      const boxes = sel.map((n) => ({ n, ...nodeBox(n) }));
      const minX = Math.min(...boxes.map((b) => b.x));
      const maxX = Math.max(...boxes.map((b) => b.x + b.w));
      const minY = Math.min(...boxes.map((b) => b.y));
      const maxY = Math.max(...boxes.map((b) => b.y + b.h));
      const horizontal = axis === "left" || axis === "right" || axis === "hcenter";

      // 1. move each node onto the shared line
      const placed = new Map<string, { x: number; y: number }>();
      for (const b of boxes) {
        let { x, y } = b;
        if (axis === "left") x = minX;
        else if (axis === "right") x = maxX - b.w;
        else if (axis === "hcenter") x = (minX + maxX) / 2 - b.w / 2;
        else if (axis === "top") y = minY;
        else if (axis === "bottom") y = maxY - b.h;
        else y = (minY + maxY) / 2 - b.h / 2;
        placed.set(b.n.id, { x, y });
      }

      // 2. de-overlap along the perpendicular axis, preserving original order
      const ordered = [...boxes].sort((a, b) => (horizontal ? a.cy - b.cy : a.cx - b.cx));
      let prevEnd = Number.NEGATIVE_INFINITY;
      for (const b of ordered) {
        const pos = placed.get(b.n.id)!;
        const size = horizontal ? b.h : b.w;
        const start = horizontal ? pos.y : pos.x;
        const shifted = Math.max(start, prevEnd + ALIGN_GAP);
        if (horizontal) pos.y = shifted;
        else pos.x = shifted;
        prevEnd = shifted + size;
      }

      setNodes((ns) =>
        ns.map((n) => {
          const pos = placed.get(n.id);
          return pos ? { ...n, x: Math.round(pos.x), y: Math.round(pos.y) } : n;
        }),
      );
      touch();
    },
    [nodes, selectedId, selectedIds, touch],
  );

  /** Even out the gaps between three or more selected nodes. */
  const distributeSelection = useCallback(
    (axis: "h" | "v") => {
      const ids = selectedIds.size ? Array.from(selectedIds) : [];
      const sel = nodes.filter((n) => ids.includes(n.id));
      if (sel.length < 3) return;
      const boxes = sel.map((n) => ({ n, ...nodeBox(n) }));
      const sorted = [...boxes].sort((a, b) => (axis === "h" ? a.cx - b.cx : a.cy - b.cy));
      const first = sorted[0];
      const last = sorted[sorted.length - 1];
      const span = axis === "h" ? last.cx - first.cx : last.cy - first.cy;
      const step = span / (sorted.length - 1);
      setNodes((ns) =>
        ns.map((n) => {
          const idx = sorted.findIndex((b) => b.n.id === n.id);
          if (idx <= 0 || idx === sorted.length - 1) return n;
          const b = sorted[idx];
          const targetCenter = (axis === "h" ? first.cx : first.cy) + step * idx;
          return axis === "h"
            ? { ...n, x: Math.round(targetCenter - b.w / 2) }
            : { ...n, y: Math.round(targetCenter - b.h / 2) };
        }),
      );
      touch();
    },
    [nodes, selectedIds, touch],
  );

  /* ---------- groups ---------- */

  /** Ask the frame to focus its title input; cleared once editing finishes. */
  const beginRenameGroup = useCallback((id: string) => {
    setSelectedGroupId(id);
    setRenamingGroupId(id);
  }, []);

  const endRenameGroup = useCallback(() => setRenamingGroupId(null), []);

  const groupSelection = useCallback(() => {
    const ids = selectedIds.size ? Array.from(selectedIds) : selectedId ? [selectedId] : [];
    if (ids.length < 1) {
      notify(i18n.t("editor.selectNodesFirst"), "Boxes");
      return;
    }
    const group = createGroupFromNodes(nodes, ids, `grp_${Date.now()}`, groups.length);
    if (!group) return;
    setGroups((g) => [...g, group]);
    setSelectedGroupId(group.id);
    touch();
    notify(i18n.t("editor.grouped", { count: ids.length }), "Boxes");
  }, [nodes, groups.length, selectedId, selectedIds, notify, touch]);

  const renameGroup = useCallback((id: string, title: string) => {
    const next = title.trim();
    if (!next) return;
    setGroups((g) => g.map((x) => (x.id === id ? { ...x, title: next } : x)));
    touch();
  }, [touch]);

  const setGroupColor = useCallback((id: string, color: NodeGroup["color"]) => {
    setGroups((g) => g.map((x) => (x.id === id ? { ...x, color } : x)));
    touch();
  }, [touch]);

  const resizeGroup = useCallback((id: string, rect: { x: number; y: number; w: number; h: number }) => {
    setGroups((g) => g.map((x) => (x.id === id ? { ...x, ...normalizeGroupRect(rect) } : x)));
    touch();
  }, [touch]);

  /** Moving a frame carries the nodes captured inside it. */
  const moveGroup = useCallback(
    (id: string, x: number, y: number, memberPositions: Record<string, { x: number; y: number }>) => {
      setGroups((g) => g.map((gr) => (gr.id === id ? { ...gr, x, y } : gr)));
      if (Object.keys(memberPositions).length) {
        setNodes((ns) => ns.map((n) => (memberPositions[n.id] ? { ...n, ...memberPositions[n.id] } : n)));
      }
      touch();
    },
    [touch],
  );

  /** Remove the frame but keep every node it contained. */
  const ungroup = useCallback(
    (id: string) => {
      setGroups((g) => g.filter((x) => x.id !== id));
      setSelectedGroupId((cur) => (cur === id ? null : cur));
      touch();
      notify(i18n.t("editor.groupRemoved"), "Boxes");
    },
    [notify, touch],
  );

  /** Select every node currently inside a frame. */
  const selectGroupMembers = useCallback(
    (id: string) => {
      const group = groups.find((g) => g.id === id);
      if (!group) return;
      const ids = nodesInGroup(nodes, group);
      setSelectedIds(new Set(ids));
      setSelectedId(ids[0] ?? null);
    },
    [groups, nodes],
  );

  /* ---------- sticky notes (UI-only comments) ---------- */

  const [comments, setComments] = useState<EditorComment[]>([]);
  const [selectedCommentId, setSelectedCommentId] = useState<string | null>(null);
  const [renamingCommentId, setRenamingCommentId] = useState<string | null>(null);

  const selectComment = useCallback((id: string | null) => {
    setSelectedCommentId(id);
    if (id) { setSelectedIds(new Set()); setSelectedId(null); }
  }, []);

  const addComment = useCallback((at: { x: number; y: number }) => {
    const id = `note-${crypto.randomUUID().slice(0, 8)}`;
    const comment = createCommentNode(id, Math.round(at.x), Math.round(at.y));
    setComments((c) => [...c, comment]);
    selectComment(id);
    setRenamingCommentId(id);
    touch();
  }, [touch, selectComment]);

  const renameComment = useCallback((id: string, text: string) => {
    setComments((c) => c.map((x) => (x.id === id ? { ...x, text } : x)));
    touch();
  }, [touch]);

  const setCommentColor = useCallback((id: string, color: NodeGroup["color"]) => {
    setComments((c) => c.map((x) => (x.id === id ? { ...x, color } : x)));
    touch();
  }, [touch]);

  const resizeComment = useCallback((id: string, rect: { x: number; y: number; w: number; h: number }) => {
    setComments((c) => c.map((x) => (x.id === id ? { ...x, ...normalizeCommentRect(rect) } : x)));
    touch();
  }, [touch]);

  const moveComment = useCallback((id: string, x: number, y: number) => {
    setComments((c) => c.map((x1) => (x1.id === id ? { ...x1, x, y } : x1)));
    touch();
  }, [touch]);

  const removeComment = useCallback((id: string) => {
    setComments((c) => c.filter((x) => x.id !== id));
    setSelectedCommentId((cur) => (cur === id ? null : cur));
    touch();
  }, [touch]);

  /* ---------- loading ---------- */

  const loadPipeline = useCallback(
    async (p: Pipeline) => {
      const ticket = ++seq.current;
      setMode("pipeline");
      setFn(null);
      setLoadError(false);
      try {
        const definition = p.draftDefinition;
        if (definition.schemaVersion !== 3) {
          setLoadError(true);
          setNodes([]);
          setEdges([]);
          return;
        }
        const hydrated = hydrateGraph(definition, definitionIndex);
        Object.assign(hydrated, expandWaypoints(hydrated.nodes, hydrated.edges));
        Object.assign(hydrated, toUiReroutes(hydrated.nodes, hydrated.edges));
        setGroups(hydrated.groups ?? []);
        setComments(hydrated.comments ?? []);

        setSelectedGroupId(null);
        setSelectedIds(new Set());        // parameter/database-driven contracts arrive only via ResolveNodeDefinition
        for (const n of hydrated.nodes) {
          if (isBackendResolvedType(n.type)) void reresolve(n);
        }
        if (ticket !== seq.current) return;
        setNodes(hydrated.nodes);
        setEdges(hydrated.edges);
        viewRef.current = hydrated.viewport;
        setPipeline(p);
        setSelectedId(null);
        setDirty(false);
        await refreshExecutionsInternal(p.id, ticket);
      } catch {
        if (ticket === seq.current) setLoadError(true);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [definitionIndex],
  );

  const loadFunction = useCallback(
    async (item: CustomFunction) => {
      const ticket = ++seq.current;
      setMode("function");
      setPipeline(null);
      setLoadError(false);
      try {
        const hydrated = hydrateGraph(item.draftDefinition, definitionIndex);
        Object.assign(hydrated, expandWaypoints(hydrated.nodes, hydrated.edges));
        Object.assign(hydrated, toUiReroutes(hydrated.nodes, hydrated.edges));
        setGroups(hydrated.groups ?? []);
        setComments(hydrated.comments ?? []);

        setSelectedGroupId(null);
        setSelectedIds(new Set());        // parameter/database-driven contracts arrive only via ResolveNodeDefinition
        for (const n of hydrated.nodes) {
          if (isBackendResolvedType(n.type)) void reresolve(n);
        }
        const withPins = applyFunctionInterface(hydrated.nodes, fnKindOf(item));
        if (ticket !== seq.current) return;
        setNodes(withPins.map((n) => (isBoundary(n.type) ? { ...n, locked: true } : n)));
        setEdges(hydrated.edges);
        viewRef.current = hydrated.viewport;
        setFn(item);
        setSelectedId(null);
        setDirty(false);
      } catch {
        if (ticket === seq.current) setLoadError(true);
      }
    },
    [definitionIndex],
  );

  const close = useCallback(() => {
    ++seq.current;
    setGroups([]);
    setSelectedGroupId(null);
    setComments([]);
    setSelectedCommentId(null);
    setMode(null);
    setPipeline(null);
    setFn(null);
    setNodes([]);
    setEdges([]);
    setLog([]);
    setExecutions([]);
    setSelectedId(null);
    setDirty(false);
    setLoadError(false);
  }, []);

  /* ---------- serialization ---------- */

  const serialize = useCallback(
    (viewport: Viewport) => {
          const backend = collapseReroutes(nodes, edges);
          return dehydrate(backend.nodes, backend.edges, viewport, groups, comments);
        },
    [nodes, edges, groups, comments],
  );

  /** Builds the updated CustomFunction payload for save/publish. */
  const serializeFunction = useCallback(
    (viewport: Viewport): CustomFunction | null => {
      if (!fn) return null;
      const entry = nodes.find((n) => n.type === "function:entry");
      const ret = nodes.find((n) => n.type === "function:return");
      const kind = fnKindFromBackend(fn);
      let inputs = fn.inputs;
      let outputs = fn.outputs;
      if (entry && ret) {
        // tools use the same ports-driven path as pure/impure — the boundary
        // pins carry spec/required/description for the agent contract
        inputs = functionPinsFromPorts(entry.outputs.filter((p) => p.kind !== "exec"), fn.inputs);
        outputs = functionPinsFromPorts(ret.inputs.filter((p) => p.kind !== "exec"), fn.outputs);
      }
      return {
        ...fn,
        kind: fnKindToBackend(kind).kind,
        mode: fnKindToBackend(kind).mode,
        inputs,
        outputs,
        draftDefinition: (() => {
          const backend = collapseReroutes(nodes, edges);
          return dehydrate(backend.nodes, backend.edges, viewport, groups, comments);
        })(),
      };
    },
    [fn, nodes, edges, groups],
  );

  /* ---------- node mutations ---------- */

  const moveNode = useCallback((id: string, x: number, y: number) => {
    setNodes((ns) => patchNode(ns, id, { x, y }));
    touch();
  }, [touch]);

  const updateField = useCallback(
    (key: string, value: unknown) => {
      if (!selectedId) return;
      setNodes((ns) =>
        ns.map((n) => (n.id === selectedId ? refreshNode(setNodeValue([n], selectedId, key, value)[0], definitionIndex) : n)),
      );
      touch();
      // authoritative re-resolution for dynamic backend-defined pins
      const node = nodes.find((n) => n.id === selectedId);
      if (node && isBackendResolvedType(node.type)) {
        void reresolve({ ...node, values: { ...node.values, [key]: value } });
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [selectedId, definitionIndex, nodes, touch],
  );

  const reresolve = useCallback(
    async (node: GraphNode) => {
      const ticket = seq.current;
      try {
        const raw = await desktop.resolveNodeDefinition({
          id: node.id,
          type: node.type,
          position: { x: node.x, y: node.y },
          data: { config: node.values },
        });
        if (ticket !== seq.current) return;
        const [localized] = localizeDefinitions([raw]);
        setNodes((ns) =>
          ns.map((n) =>
            n.id === node.id
              ? {
                ...n,
                title: localized.label,
                icon: localized.icon,
                group: localized.category,
                summary: localized.description,
                inputs: (localized.inputs ?? []).map(portFromBackendPin),
                outputs: (localized.outputs ?? []).map(portFromBackendPin),
              }
              : n,
          ),
        );
      } catch {
        /* keep locally derived pins */
      }
    },
    [],
  );

  const renameNode = useCallback((id: string, _title: string) => {
    // node titles come from the localized catalog; renaming is not persisted
    void id;
  }, []);

  const toggleNodeStatus = useCallback((id: string) => {
    setNodes((ns) =>
      ns.map((n) => (n.id === id ? { ...n, status: n.status === "idle" ? "done" : "idle" } : n)),
    );
  }, []);

  const updateNodePorts = useCallback(
    (nodeId: string, inputs: Port[], outputs: Port[]) => {
      setNodes((ns) => patchNode(ns, nodeId, { inputs, outputs }));
      touch();
    },
    [touch],
  );

  const copyNode = useCallback(
    (id: string) => {
      const node = nodes.find((n) => n.id === id);
      if (!node) return;
      clipboard.current = node;
      notify(i18n.t("editor.nodeCopied"), "Copy");
    },
    [nodes, notify],
  );

  const deleteSelected = useCallback(() => {
    const requested = selectedIds.size ? Array.from(selectedIds) : selectedId ? [selectedId] : [];
    if (!requested.length) return;
    const removable = requested.filter((id) => !nodes.find((n) => n.id === id)?.locked);
    if (!removable.length) {
      notify(i18n.t("editor.boundaryLocked"), "Lock");
      return;
    }
    setNodes((ns) => ns.filter((n) => !removable.includes(n.id)));
    setEdges((current) => {
      let next = current;
      for (const id of removable) {
        const target = nodes.find((n) => n.id === id);
        next = isReroute(target) ? bridgeRerouteOnDelete(next, id) : removeEdgesForNode(next, id);
      }
      return next;
    });
    selectOnly(null);
    touch();
  }, [nodes, selectedId, selectedIds, notify, touch, selectOnly]);

  const duplicateSelected = useCallback(() => {
    const ids = selectedIds.size ? Array.from(selectedIds) : selectedId ? [selectedId] : [];
    const originals = nodes.filter((n) => ids.includes(n.id) && !n.locked);
    if (!originals.length) return;
    const idMap = new Map(originals.map((n) => [n.id, makeNodeId(n.type)]));
    const copies = originals.map((n) => duplicateNode(n, idMap.get(n.id)!));
    const copiedEdges = edges
      .filter((e) => idMap.has(e.from.node) && idMap.has(e.to.node))
      .map((e) => ({
        ...e,
        id: `e-${crypto.randomUUID().slice(0, 8)}`,
        from: { ...e.from, node: idMap.get(e.from.node)! },
        to: { ...e.to, node: idMap.get(e.to.node)! },
      }));
    const copiedIds = new Set(copies.map((n) => n.id));
    setNodes((ns) => [...ns, ...copies]);
    setEdges((es) => [...es, ...copiedEdges]);
    setSelectedIds(copiedIds);
    setSelectedId(copies[0].id);
    touch();
  }, [nodes, edges, selectedId, selectedIds, touch]);

  const addNode = useCallback(
    (item: LibraryItem, _group: string, at: { x: number; y: number }) => {
      if (item.functionId) {
        notify(i18n.t("editor.functionPlacePending"), "Braces");
        return;
      }
      const definition = item.type ? definitionIndex[item.type] : undefined;
      if (!definition) {
        notify(i18n.t("editor.nodeUnavailable"), "AlertTriangle");
        return;
      }
      const node = refreshNode(buildFromDefinition(definition, at), definitionIndex);
      setNodes((ns) => [...ns, node]);
      setSelectedId(node.id);
      touch();
      if (isBackendResolvedType(node.type)) void reresolve(node);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [definitionIndex, notify, touch],
  );

  /* ---------- edge mutations ---------- */

  const connect = useCallback(
    (from: { node: string; port: string }, to: { node: string; port: string }, _kind: PortKind) => {
      const sourceNode = nodes.find((n) => n.id === from.node);
      const targetNode = nodes.find((n) => n.id === to.node);
      const source = sourceNode?.outputs.find((p) => p.id === from.port);
      const target = targetNode?.inputs.find((p) => p.id === to.port);
      if (!source || !target) return;
      if (!typesCompatible(source, target)) {
        notify(i18n.t("editor.incompatibleConnection"), "X");
        return;
      }
      if (edgeExists(edges, from, to)) return;
      const incoming = edges.filter((e) => e.to.node === to.node && e.to.port === to.port).length;
      if (target.maxConnections !== undefined && incoming >= target.maxConnections) {
        notify(i18n.t("editor.maxConnectionsReached"), "X");
        return;
      }
      const kind: PortKind = source.kind === "exec" ? "exec" : "data";
      const dataType = source.kind === "exec" ? "exec" : source.dataType ?? "any";

      // an input pin holds a single wire — replace whatever was there
      let nextEdges: Edge[] = [...detachInput(edges, to), { id: `e-${crypto.randomUUID().slice(0, 8)}`, from, to, kind, dataType }];
      let nextNodes = nodes;

      // a reroute adopts the incoming type and pushes it downstream
      if (isReroute(targetNode)) {
        const propagated = propagateRerouteType(nodes, nextEdges, to.node, dataType);
        nextNodes = propagated.nodes;
        nextEdges = propagated.edges;
      }

      setNodes(nextNodes);
      setEdges(nextEdges);
      touch();
    },
    [nodes, edges, notify, touch],
  );

  const removeEdge = useCallback(
    (id: string) => {
      const dropped = edges.find((e) => e.id === id);
      const next = edges.filter((e) => e.id !== id);
      if (!dropped) return;
      // a reroute that lost its source goes back to untyped
      const cleared = clearRerouteTypeIfOrphaned(nodes, next, dropped.to.node);
      setNodes(cleared.nodes);
      setEdges(cleared.edges);
      touch();
    },
    [edges, nodes, touch],
  );

  /** Drop a reroute onto an existing wire, splicing it in the middle. */
  const insertReroute = useCallback(
    (edgeId: string, at: { x: number; y: number }) => {
      const edge = edges.find((e) => e.id === edgeId);
      if (!edge) return;
      const id = makeNodeId("reroute");
      const node = createRerouteNode(id, Math.round(at.x), Math.round(at.y), edge.dataType ?? "any");
      setNodes((ns) => [...ns, node]);
      setEdges((es) => [
        ...es.filter((e) => e.id !== edgeId),
        { id: `e-${crypto.randomUUID().slice(0, 8)}`, from: edge.from, to: { node: id, port: REROUTE_IN }, kind: edge.kind, dataType: edge.dataType },
        { id: `e-${crypto.randomUUID().slice(0, 8)}`, from: { node: id, port: REROUTE_OUT }, to: edge.to, kind: edge.kind, dataType: edge.dataType },
      ]);
      setSelectedId(id);
      touch();
    },
    [edges, touch],
  );

  const removeEdgesFor = useCallback(
    (nodeId: string, portId: string) => {
      setEdges((es) =>
        portId ? removeEdgesForPort(es, nodeId, portId) : es.filter((e) => e.from.node !== nodeId && e.to.node !== nodeId),
      );
      touch();
    },
    [touch],
  );

  /* ---------- function boundary editing ---------- */

  const setFunctionKind = useCallback(
    (kind: FunctionKind) => {
      if (!fn) return;
      setNodes((ns) => applyFunctionKind(ns, kind));
      setFn((f) => {
        if (!f) return f;
        const mapped = fnKindToBackend(kind);
        return { ...f, kind: mapped.kind, mode: mapped.mode };
      });
      touch();
    },
    [fn, touch],
  );

  const boundaryPins = useCallback((): { entryOutputs: Port[]; returnInputs: Port[] } => {
    const entry = nodes.find((n) => n.type === "function:entry");
    const ret = nodes.find((n) => n.type === "function:return");
    return {
      entryOutputs: entry?.outputs.filter((p) => p.kind !== "exec") ?? [],
      returnInputs: ret?.inputs.filter((p) => p.kind !== "exec") ?? [],
    };
  }, [nodes]);

  /* ---------- execution ---------- */

  const triggerNode = useCallback(() => {
    return (
      nodes.find((n) => n.type === "trigger:button") ??
      nodes.find((n) => n.type.startsWith("trigger:")) ??
      null
    );
  }, [nodes]);

  const refreshExecutionsInternal = async (pipelineId: string, ticket: number) => {
    try {
      const list = await desktop.listExecutions(pipelineId);
      if (ticket !== seq.current) return;
      setExecutions(list);
      const latest = list[0];
      if (latest) applyExecution(latest);
    } catch {
      /* history is optional */
    }
  };

  const applyExecution = useCallback(
    (execution: Execution) => {
      const runs = execution.nodeRuns ?? [];
      setNodes((ns) =>
        ns.map((n) => {
          const run = runs.find((r) => r.nodeId === n.id);
          if (!run) return n;
          return {
            ...n,
            status: applyRunStatus(run.status),
            lastRun: {
              status: applyRunStatus(run.status),
              error: run.error,
            },
          };
        }),
      );
      const logs = runs
        .map((r) => nodeRunToLog(r, nodes))
        .sort((a, b) => Date.parse(b.time) - Date.parse(a.time));
      setLog(logs);
    },
    [nodes],
  );

  const run = useCallback(async () => {
    if (!pipeline) return;
    const trigger = triggerNode();
    if (!trigger) {
      notify(i18n.t("editor.addTrigger"), "Zap");
      return;
    }
    setBusy(true);
    try {
      const draft = serialize(viewRef.current);
      const saved = await desktop.savePipeline({ ...pipeline, draftDefinition: draft });
      setPipeline(saved);
      setNodes(resetStatuses);
      const execution = await desktop.runPipelineDraft(saved.id, trigger.id);
      if (execution.error) notify(execution.error, "AlertTriangle");
      else notify(i18n.t("editor.runFinished"), "Check");
      applyExecution(execution);
      await refreshExecutionsInternal(saved.id, seq.current);
    } catch (err) {
      notify(err instanceof Error ? err.message : i18n.t("editor.runFailed"), "AlertTriangle");
    } finally {
      setBusy(false);
    }
  }, [pipeline, triggerNode, serialize, applyExecution, notify]);

  const stop = useCallback(async () => {
    if (!pipeline) return;
    try {
      await desktop.cancelPipelineExecution(pipeline.id);
    } catch {
      notify(i18n.t("pipelines.stopFailed"), "AlertTriangle");
    }
    await refreshExecutionsInternal(pipeline.id, seq.current);
  }, [pipeline, notify]);

  const loadExecution = useCallback(
    (execution: Execution) => {
      applyExecution(execution);
    },
    [applyExecution],
  );

  const running = pipeline ? Boolean(runningMap[pipeline.id]) || busy : busy;

  return {
    // identity
    mode, pipeline, fn, loadError,
    // state
    nodes, edges, log, executions, selected, selectedId, running, dirty, busy,
    viewRef,
    // selection + dirty
    setSelectedId, setSelectedIds, setDirty,
    selectedIds,
    selectOnly, clearSelection, selectAll, toggleSelect, selectMarquee,
    // groups (UI-only)
    groups, selectedGroupId, setSelectedGroupId, groupSelection, renameGroup,
    setGroupColor, resizeGroup, moveGroup, ungroup, selectGroupMembers,
    renamingGroupId, beginRenameGroup, endRenameGroup,
    // sticky notes (UI-only)
    comments, selectedCommentId, setSelectedCommentId, renamingCommentId, setRenamingCommentId,
    addComment, renameComment, setCommentColor, resizeComment, moveComment, removeComment,    // multi-selection layout
    moveNodes, alignSelection, distributeSelection,
    // node ops
    moveNode, updateField, renameNode, toggleNodeStatus, updateNodePorts,
    copyNode, deleteSelected, duplicateSelected, addNode,
    // edge ops
    connect, removeEdge, removeEdgesFor, insertReroute,
    // function-specific
    setFunctionKind, boundaryPins,
    // lifecycle
    loadPipeline, loadFunction, close, serialize, serializeFunction,
    // execution
    run, stop, loadExecution,
  };
}

export type GraphEditor = ReturnType<typeof useGraphEditor>;

/* ------------------------------------------------------------------ */
/* helpers                                                             */
/* ------------------------------------------------------------------ */

function isBoundary(type: string) {
  return type === "function:entry" || type === "function:return";
}

function portFromBackendPin(p: NodePort): Port {
  return {
    id: p.id,
    label: p.label,
    kind: p.kind === "exec" ? "exec" : "data",
    dataType: p.kind === "exec" ? "exec" : mapDataType(p.dataType),
    spec: p.type,
    maxConnections: p.maxConnections,
  };
}

function fnKindOf(fn: CustomFunction): { kind: FunctionKind; inputs: Port[]; outputs: Port[] } {
  const kind = fnKindFromBackend(fn);
  const toPort = (pin: CustomFunction["inputs"][number]): Port => ({
    id: pin.id,
    label: pin.name,
    kind: "data",
    dataType: mapDataType(pin.dataType),
    spec: pin.type,
    description: pin.description,
    required: pin.required,
  });
  return { kind, inputs: fn.inputs.map(toPort), outputs: fn.outputs.map(toPort) };
}

export function buildFromDefinition(definition: NodeDefinition, at: { x: number; y: number }): GraphNode {
  const snap = (v: number) => Math.round(v / 8) * 8;
  return {
    id: `${definition.type.replace(":", "-")}-${crypto.randomUUID().slice(0, 8)}`,
    type: definition.type,
    title: definition.label,
    icon: definition.icon,
    group: definition.category,
    summary: definition.description,
    x: snap(at.x),
    y: snap(at.y),
    status: "idle",
    inputs: [], // filled by refreshNode below via caller when needed
    outputs: [],
    fields: [],
    values: structuredClone(definition.defaultConfig ?? {}),
  };
}



























