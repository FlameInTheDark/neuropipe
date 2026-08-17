import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ComponentType,
} from "react";
import {
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  Background,
  Controls,
  Handle,
  MarkerType,
  Panel,
  Position,
  ReactFlow,
  ReactFlowProvider,
  reconnectEdge,
  type Connection,
  type EdgeChange,
  type NodeChange,
  type NodeProps,
  type ReactFlowInstance,
} from "@xyflow/react";
import dynamicIconImports from "lucide-react/dynamicIconImports";
import {
  ArrowLeft,
  BookOpen,
  Bot,
  Check,
  ChevronRight,
  CirclePlay,
  Copy,
  FileCode2,
  LayoutGrid,
  Loader2,
  Magnet,
  MousePointer2,
  PanelLeft,
  PanelRight,
  Play,
  Plus,
  Save,
  Search,
  Send,
  Square,
  Trash2,
  Undo2,
  UploadCloud,
  X,
  Zap,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { BlueprintContextMenu } from "@/components/BlueprintContextMenu";
import { contextMenuPosition } from "@/components/ContextMenu";
import {
  FieldOutputsEditor,
  HtmlExtractionsEditor,
  ObjectFieldsEditor,
} from "@/components/BlueprintDataMappingsEditor";
import { BlueprintSwitchCasesEditor } from "@/components/BlueprintSwitchCasesEditor";
import { FormBuilderEditor } from "@/components/BlueprintFormBuilderEditor";
import { BlueprintNodeLibrary } from "@/components/BlueprintNodeLibrary";
import { BlueprintNodeCard } from "@/components/BlueprintNodeCard";
import { JavaScriptCodeControl } from "@/components/JavaScriptCodeControl";
import { TextEditorExpandButton, TextEditorField } from "@/components/TextEditorField";
import { DatabaseSelectControl, SQLCodeControl } from "@/components/SQLCodeControl";
import { IconAppearancePicker, LucideIconPicker } from "@/components/LucideIconPicker";
import { ShortcutRecorder } from "@/components/ShortcutRecorder";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tooltip } from "@/components/ui/tooltip";
import { desktop } from "@/lib/bridge";
import { dataPinColor, nodePinColor } from "@/lib/node-pins";
import {
  routeOptionsFromValue,
  resolveConfigDrivenInputs,
  resolveConfigDrivenOutputs,
  type RouteOptionValue,
} from "@/lib/blueprint-dynamic-pins";
import { usePersistedChoice } from "@/lib/preferences";
import { cn, formatDate } from "@/lib/utils";
import type {
  ConfigField,
  DataField,
  DataType,
  Execution,
  FlowEdge,
  FlowNode,
  NodeDefinition,
  NodePort,
  NodeRun,
  Pipeline,
  DocumentationReference,
} from "@/lib/types";
import { isTypeAssignable, typeSpecFromDataType } from "@/lib/type-spec";
import { useUIStore } from "@/stores/ui";
import i18n from "@/i18n";
import { localizeNodeDefinitions } from "@/i18n/node-catalog";
import { useTranslation } from "react-i18next";
import { TwitchIdentitySelect } from "@/components/TwitchIdentitySelect";
import { BlueprintWaypointEdge, waypointMoveEvent, waypointRemoveEvent } from "@/components/BlueprintWaypointEdge";

interface EditorNodeData {
  type: string;
  config: Record<string, unknown>;
  label?: string;
  icon?: string;
  inputs?: NodePort[];
  outputs?: NodePort[];
        // Dynamic modules keep the backend-resolved metadata with the editor node.
        // It is intentionally omitted when serialising a FlowNode.
        resolvedDefinition?: NodeDefinition;
  lastRun?: NodeRun;
}
type EditorNode = FlowNode & { data: EditorNodeData };

const gridSnapModes = ["on", "off"] as const;
const editorSnapGrid: [number, number] = [20, 20];
interface CanvasMenu {
  x: number;
  y: number;
  position: { x: number; y: number };
  nodeID?: string;
  edgeID?: string;
  edgeKind?: string;
  source?: string;
  sourceHandle?: string | null;
}
interface ReconnectState {
  edgeID: string;
  allowed: boolean;
  completed: boolean;
}

const contextOnlyNodeTypes = new Set([
  "function:entry",
  "function:return",
  "function:input",
  "function:output",
]);
const backendResolvedNodeTypes = new Set([
  "twitch:event",
  "data:get_global_variable",
  "flow:set_global_variable",
  "action:sql",
]);
const defaultEdgeOptions = { reconnectable: "target" as const };
const nodeLibraryCollapsedCategoriesKey =
  "neuropipe.node-library.collapsed-categories.v1";

function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function normalizeDefinition(definition: NodeDefinition): NodeDefinition {
  return {
    ...definition,
    inputs: asArray(definition.inputs),
    outputs: asArray(definition.outputs),
    fields: asArray(definition.fields),
    capabilities: asArray(definition.capabilities),
    defaultConfig: isRecord(definition.defaultConfig)
      ? definition.defaultConfig
      : {},
  };
}

function compatiblePins(source: NodePort, target: NodePort) {
  if (source.kind === "data" && source.type && target.type) {
    return isTypeAssignable(source.type, target.type);
  }
  return (
    source.kind === target.kind &&
    (source.kind === "exec" ||
      source.dataType === "any" ||
      source.dataType === target.dataType)
  );
}

function iconFor(type: string) {
  if (type.startsWith("trigger:")) return Zap;
  if (type.startsWith("llm:")) return Bot;
  if (type.startsWith("action:file")) return FileCode2;
  return CirclePlay;
}

function NodeIcon({
  name,
  fallback: Fallback,
}: {
  name?: string;
  fallback: ComponentType<{ className?: string }>;
}) {
  const [Icon, setIcon] = useState<ComponentType<{ className?: string }>>(
    () => Fallback,
  );
  useEffect(() => {
    const loader = name
      ? dynamicIconImports[name as keyof typeof dynamicIconImports]
      : undefined;
    if (!loader) {
      setIcon(() => Fallback);
      return;
    }
    void loader()
      .then((module) =>
        setIcon(() => module.default as ComponentType<{ className?: string }>),
      )
      .catch(() => setIcon(() => Fallback));
  }, [name, Fallback]);
  return <Icon className="size-3 text-zinc-300" />;
}

function NeuropipeNode({ data, selected }: NodeProps<EditorNode>) {
  const { t } = useTranslation();
  const node = data as EditorNodeData;
  const Icon = iconFor(node.type);
  const inputs = node.inputs ?? [];
  const outputs = node.outputs ?? [];
  if (node.type === "visual:comment")
    return (
      <div
        className={cn(
          "min-w-56 rounded-lg border border-dashed bg-zinc-900/85 px-4 py-3 shadow-xl",
          selected ? "border-zinc-300 ring-2 ring-white/10" : "border-zinc-600",
        )}
      >
        <p className="text-xs font-semibold text-zinc-200">
          {String(node.config.title ?? t("editor.comment"))}
        </p>
        <p className="mt-1 max-w-60 whitespace-pre-wrap text-[11px] leading-4 text-zinc-500">
          {String(node.config.body ?? "") || t("editor.commentEmpty")}
        </p>
      </div>
    );
  if (node.type === "flow:reroute" || node.type === "data:reroute")
    return (
      <RerouteNode
        inputs={inputs}
        outputs={outputs}
        selected={selected}
        exec={node.type === "flow:reroute"}
      />
    );
  const runTone =
    node.lastRun?.status === "completed"
      ? "border-emerald-500/70 shadow-emerald-950/30"
      : node.lastRun?.status === "failed"
        ? "border-red-500/70 shadow-red-950/30"
        : node.lastRun?.status === "running"
          ? "border-amber-400/70 shadow-amber-950/30"
          : "border-zinc-700";
  const runDot =
    node.lastRun?.status === "completed"
      ? "bg-emerald-400"
      : node.lastRun?.status === "failed"
        ? "bg-red-400"
        : node.lastRun?.status === "running"
          ? "animate-pulse bg-amber-400"
          : "";
  return (
    <BlueprintNodeCard
      label={node.label || node.type.replace(":", " · ")}
      icon={<NodeIcon name={node.icon} fallback={Icon} />}
      summary={summaryFor(node)}
      inputs={inputs}
      outputs={outputs}
      selected={selected}
      tone={runTone}
      status={
        node.lastRun ? (
          <Tooltip content={node.lastRun.status} side="bottom">
            <span className={cn("size-2 shrink-0 rounded-full", runDot)} />
          </Tooltip>
        ) : undefined
      }
    />
  );
}

function RerouteNode({
  inputs,
  outputs,
  selected,
  exec,
}: {
  inputs: NodePort[];
  outputs: NodePort[];
  selected: boolean;
  exec: boolean;
}) {
  const { t } = useTranslation();
  const input = inputs.find((pin) => pin.kind === (exec ? "exec" : "data"));
  const output = outputs.find((pin) => pin.kind === (exec ? "exec" : "data"));
  if (!input || !output) return null;
  const color = nodePinColor(input);
  return (
    <div
      aria-label={t("editor.reroute")}
      className={cn(
        "relative flex size-5 items-center justify-center rounded-full border bg-zinc-950 shadow-lg",
        selected ? "border-zinc-100 ring-2 ring-white/20" : "border-zinc-500",
      )}
    >
      <Handle
        id={input.id}
        type="target"
        position={Position.Left}
        className={exec ? "!h-3 !w-3 !rounded-sm" : "!size-2.5"}
        style={{ background: color, left: 0 }}
      />
      <span
        className={cn(
          exec ? "size-1.5 rounded-sm bg-zinc-100" : "size-1.5 rounded-full",
        )}
        style={exec ? undefined : { background: color }}
      />
      <Handle
        id={output.id}
        type="source"
        position={Position.Right}
        className={exec ? "!h-3 !w-3 !rounded-sm" : "!size-2.5"}
        style={{ background: color, right: 0 }}
      />
    </div>
  );
}

function formatDataType(dataType?: DataType) {
  const key = dataType === "text" || dataType === "number" || dataType === "boolean" || dataType === "object" || dataType === "list" ? dataType : "any";
  return i18n.t(`editor.${key}`);
}

function KnownOutputFields({ pins }: { pins: NodePort[] }) {
  const { t } = useTranslation();
  const shapedPins = pins.filter((pin) => asArray(pin.fields).length > 0);
  if (shapedPins.length === 0) return null;
  return (
    <div className="mb-5 rounded-md border border-zinc-800 bg-zinc-900/30 p-3">
      <p className="text-[10px] font-semibold uppercase tracking-[.12em] text-zinc-500">
        {t("editor.knownOutputFields")}
      </p>
      <div className="mt-2 space-y-2">
        {shapedPins.map((pin) => (
          <div key={pin.id}>
            <p className="text-[11px] font-medium text-zinc-300">{pin.label}</p>
            <div className="mt-1 space-y-1">
              {asArray(pin.fields).map((field) => (
                <div
                  key={field.path}
                  className="flex items-start justify-between gap-3 font-mono text-[10px]"
                >
                  <Tooltip content={field.description || field.path} side="top" align="start">
                    <span className="min-w-0 truncate text-zinc-400">{field.path}</span>
                  </Tooltip>
                  <span className="shrink-0 text-zinc-600">
                    {formatDataType(field.dataType)}
                    {field.optional ? "?" : ""}
                  </span>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function summaryFor(node: EditorNodeData) {
  const config = node.config ?? {};
  return String(
    config.label ??
      config.prompt ??
      config.url ??
      config.command ??
      config.cron ??
      i18n.t("editor.configureNode"),
  ).slice(0, 42);
}

function withExecutionResults(
  nodes: EditorNode[],
  execution?: Execution,
): EditorNode[] {
  const runsByNodeID = new Map(
    asArray(execution?.nodeRuns).map((nodeRun) => [nodeRun.nodeId, nodeRun]),
  );
  return nodes.map((node) => ({
    ...node,
    data: { ...node.data, lastRun: runsByNodeID.get(node.id) },
  }));
}

function withExecutionEdges(
  edges: FlowEdge[],
  execution?: Execution,
): FlowEdge[] {
  const completed = new Set(
    asArray(execution?.nodeRuns)
      .filter((nodeRun) => nodeRun.status === "completed")
      .map((nodeRun) => nodeRun.nodeId),
  );
  return edges.map((edge) =>
    edge.kind === "exec" && completed.has(edge.source)
      ? {
          ...edge,
          animated: true,
          style: { ...edge.style, stroke: "#86efac", strokeWidth: 2.5 },
          markerEnd: { type: MarkerType.ArrowClosed, color: "#86efac" },
        }
      : edge,
  );
}

// Wire visuals derive from the source pin and are deliberately never
// persisted, so every path that restores edges rebuilds them the same way.
function baseEdgeVisuals(pin: NodePort | undefined) {
  const exec = pin?.kind === "exec";
  return {
    animated: exec,
    markerEnd: exec
      ? { type: MarkerType.ArrowClosed, color: "#fafafa" }
      : undefined,
    style: {
      stroke: exec ? "#fafafa" : pin?.color || "#71717a",
      strokeWidth: exec ? 2 : 1.4,
    },
  };
}

function withBaseEdgeStyles(
  edges: FlowEdge[],
  nodes: EditorNode[],
): FlowEdge[] {
  return edges.map((edge) => ({
    ...edge,
    ...baseEdgeVisuals(
      nodes
        .find((node) => node.id === edge.source)
        ?.data.outputs?.find((item) => item.id === edge.sourceHandle),
    ),
  }));
}

// A data reroute mirrors the pin feeding it: the output takes the connected
// wire's exact type and colour, so downstream validation sees the true type.
function rerouteFeedingPin(
  node: EditorNode,
  nodes: EditorNode[],
  edges: FlowEdge[],
): NodePort | undefined {
  const incoming = edges.find(
    (edge) =>
      edge.target === node.id &&
      edge.targetHandle === "value" &&
      edge.kind !== "tool",
  );
  if (!incoming) return undefined;
  const pin = nodes
    .find((item) => item.id === incoming.source)
    ?.data.outputs?.find((output) => output.id === incoming.sourceHandle);
  return pin && pin.kind === "data" ? pin : undefined;
}

const nodeTypes = { neuropipe: NeuropipeNode };
const edgeTypes = { waypoint: BlueprintWaypointEdge };

export function PipelineEditor({
  pipelineID,
  definitions,
  onRefresh,
}: {
  pipelineID: string;
  definitions: NodeDefinition[];
  onRefresh: () => Promise<void>;
}) {
  const safeDefinitions = useMemo(
    () => asArray(definitions).map(normalizeDefinition),
    [definitions],
  );
  return (
    <ReactFlowProvider>
      <EditorContents
        pipelineID={pipelineID}
        definitions={safeDefinitions}
        onRefresh={onRefresh}
      />
    </ReactFlowProvider>
  );
}

function EditorContents({
  pipelineID,
  definitions,
  onRefresh,
}: {
  pipelineID: string;
  definitions: NodeDefinition[];
  onRefresh: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const { setScreen, setError } = useUIStore();
  const [pipeline, setPipeline] = useState<Pipeline>();
  const [nodes, setNodes] = useState<EditorNode[]>([]);
  const [edges, setEdges] = useState<FlowEdge[]>([]);
  const [selectedID, setSelectedID] = useState<string>();
  const [editingName, setEditingName] = useState(false);
  const [nameDraft, setNameDraft] = useState("");
  const [search, setSearch] = useState("");
  const [showPalette, setShowPalette] = useState(true);
  const [showInspector, setShowInspector] = useState(true);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState("");
  const [history, setHistory] = useState<Execution[]>([]);
  const [runLogKey, setRunLogKey] = useState(0);
  const [gridSnapMode, setGridSnapMode] = usePersistedChoice(
    "neuropipe.pipeline-editor.grid-snap.v1",
    gridSnapModes,
    "on",
  );
  const [flow, setFlow] = useState<ReactFlowInstance<EditorNode, FlowEdge>>();
  const [menu, setMenu] = useState<CanvasMenu>();
  const [menuSearch, setMenuSearch] = useState("");
  const [connecting, setConnecting] = useState<{
    source: string;
    sourceHandle: string | null;
  }>();
  const canvasRef = useRef<HTMLDivElement>(null);
        useEffect(() => {
                const move = (event: Event) => {
                        const detail = (event as CustomEvent<{ edgeID: string; index: number; position: { x: number; y: number } }>).detail;
                        if (!detail) return;
                        setEdges((current) => current.map((edge) => edge.id !== detail.edgeID ? edge : { ...edge, waypoints: (edge.waypoints ?? []).map((point, index) => index === detail.index ? detail.position : point) }));
                        setDirty(true);
                };
                const remove = (event: Event) => {
                        const detail = (event as CustomEvent<{ edgeID: string; index: number }>).detail;
                        if (!detail) return;
                        setEdges((current) => current.map((edge) => edge.id !== detail.edgeID ? edge : { ...edge, waypoints: (edge.waypoints ?? []).filter((_, index) => index !== detail.index) }));
                        setDirty(true);
                };
                window.addEventListener(waypointMoveEvent, move);
                window.addEventListener(waypointRemoveEvent, remove);
                return () => { window.removeEventListener(waypointMoveEvent, move); window.removeEventListener(waypointRemoveEvent, remove); };
        }, []);
  const reconnectRef = useRef<ReconnectState>();
  const snapToGrid = gridSnapMode === "on";
  const snapPosition = useCallback(
    (position: { x: number; y: number }) =>
      snapToGrid
        ? {
            x: Math.round(position.x / editorSnapGrid[0]) * editorSnapGrid[0],
            y: Math.round(position.y / editorSnapGrid[1]) * editorSnapGrid[1],
          }
        : position,
    [snapToGrid],
  );

  const applyExecution = useCallback(
    (execution?: Execution) =>
      setNodes((current) => withExecutionResults(current, execution)),
    [],
  );

  const load = useCallback(async () => {
    try {
      setBusy("load");
      const [nextPipeline, nextHistory] = await Promise.all([
        desktop.getPipeline(pipelineID),
        desktop.listExecutions(pipelineID),
      ]);
      setPipeline(nextPipeline);
                const hydrated = withExecutionResults(
                        hydrateNodes(asArray(nextPipeline.draftDefinition?.nodes), definitions),
                        asArray(nextHistory)[0],
                );
                setNodes(hydrated);
                const dynamic = hydrated.filter((node) => backendResolvedNodeTypes.has(node.data.type));
                if (dynamic.length > 0) {
                        void Promise.all(dynamic.map(async (node) => {
                                const definition = await desktop.resolveNodeDefinition({ id: node.id, type: node.data.type, position: node.position, data: { config: node.data.config } });
                                return { node, definition: localizeNodeDefinitions([definition], i18n.language)[0] ?? definition };
                        })).then((resolved) => {
                                const byID = new Map(resolved.map((item) => [item.node.id, item.definition]));
                                const applyResolved = (node: EditorNode) => {
                                        const definition = byID.get(node.id);
                                        return definition ? { ...node, data: { ...node.data, resolvedDefinition: definition, inputs: resolveInputs(definition, node.data.config), outputs: resolveOutputs(definition, node.data.config) } } : node;
                                };
                                setNodes((current) => current.map(applyResolved));
                                // Resolved definitions can change pin colours, so wires that
                                // originate from them are restyled from the refreshed pins.
                                setEdges((current) => withExecutionEdges(withBaseEdgeStyles(current, hydrated.map(applyResolved)), asArray(nextHistory)[0]));
                        }).catch(() => { /* validation reports the backend resolver error on save/publish */ });
                }
      setEdges(
        withExecutionEdges(
          withBaseEdgeStyles(
            asArray(nextPipeline.draftDefinition?.edges).map((edge) =>
              edge.waypoints && edge.waypoints.length > 0
                ? { ...edge, type: "waypoint" }
                : edge,
            ),
            hydrated,
          ),
          asArray(nextHistory)[0],
        ),
      );
      setHistory(asArray(nextHistory));
      setDirty(false);
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : t("editor.loadFailed"),
      );
    } finally {
      setBusy("");
    }
  }, [definitions, pipelineID, setError, t]);
  useEffect(() => {
    void load();
  }, [load]);
  // Data reroutes derive their output type from the wire feeding them, so the
  // pin follows connects, disconnects, and upstream retypes, and downstream
  // wires re-colour to match. The input pin keeps its single-connection limit;
  // the output fans out like any node pin.
  useEffect(() => {
    if (!nodes.some((node) => node.data.type === "data:reroute")) return;
    let changed = false;
    const nextNodes = nodes.map((node) => {
      if (node.data.type !== "data:reroute") return node;
      const outputs = node.data.outputs ?? [];
      const output = outputs[0];
      if (!output) return node;
      const feeding = rerouteFeedingPin(node, nodes, edges);
      const dataType = feeding?.dataType ?? "any";
      const type = feeding?.type ?? typeSpecFromDataType(dataType);
      const color = feeding?.color ?? dataPinColor(dataType);
      if (
        output.dataType === dataType &&
        JSON.stringify(output.type) === JSON.stringify(type) &&
        output.color === color
      ) {
        return node;
      }
      changed = true;
      return {
        ...node,
        data: {
          ...node.data,
          outputs: [{ ...output, dataType, type, color }, ...outputs.slice(1)],
        },
      };
    });
    if (!changed) return;
    setNodes(nextNodes);
    setEdges((current) => withBaseEdgeStyles(current, nextNodes));
  }, [nodes, edges]);
  useEffect(() => {
    const keydown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
        event.preventDefault();
        void save();
      }
    };
    window.addEventListener("keydown", keydown);
    return () => window.removeEventListener("keydown", keydown);
  });

  const selected = nodes.find((node) => node.id === selectedID);
  const selectedDefinition = selected?.data.resolvedDefinition ?? definitions.find(
    (definition) => definition.type === selected?.data.type,
  );
  const selectedSourceFields = useMemo(() => {
    if (
      selected?.data.type !== "data:get_field" &&
      selected?.data.type !== "data:break_object"
    )
      return [];
    const sourceEdge = edges.find(
      (edge) =>
        edge.target === selected.id &&
        edge.targetHandle === "source" &&
        edge.kind === "data",
    );
    if (!sourceEdge) return [];
    const source = nodes.find((node) => node.id === sourceEdge.source);
    const sourcePin = source?.data.outputs?.find(
      (pin) => pin.id === sourceEdge.sourceHandle,
    );
    return asArray(sourcePin?.fields);
  }, [edges, nodes, selected]);
  const filteredDefinitions = useMemo(
    () =>
      definitions.filter(
        (definition) =>
          !contextOnlyNodeTypes.has(definition.type) &&
          `${definition.label} ${definition.category} ${definition.description}`
            .toLowerCase()
            .includes(search.toLowerCase()),
      ),
    [definitions, search],
  );

  const onNodesChange = useCallback((changes: NodeChange<EditorNode>[]) => {
    setNodes((current) => applyNodeChanges(changes, current));
    if (changes.some((change) => change.type !== "select")) setDirty(true);
  }, []);
  const onEdgesChange = useCallback((changes: EdgeChange<FlowEdge>[]) => {
    setEdges((current) => applyEdgeChanges(changes, current));
    if (changes.some((change) => change.type !== "select")) setDirty(true);
  }, []);
  const isValidConnection = useCallback(
    (connection: Connection | FlowEdge) => {
      if (
        !connection.source ||
        !connection.target ||
        !connection.sourceHandle ||
        !connection.targetHandle
      )
        return false;
      const source = nodes.find((node) => node.id === connection.source);
      const target = nodes.find((node) => node.id === connection.target);
      const sourcePin = source?.data.outputs?.find(
        (pin) => pin.id === connection.sourceHandle,
      );
      const targetPin = target?.data.inputs?.find(
        (pin) => pin.id === connection.targetHandle,
      );
      if (!sourcePin || !targetPin || !compatiblePins(sourcePin, targetPin))
        return false;
      const reconnectingID = reconnectRef.current?.edgeID;
      const incoming = edges.filter(
        (edge) =>
          edge.id !== reconnectingID &&
          edge.target === connection.target &&
          edge.targetHandle === connection.targetHandle,
      ).length;
      return !targetPin.maxConnections || incoming < targetPin.maxConnections;
    },
    [edges, nodes],
  );
  const onConnect = useCallback(
    (connection: Connection) => {
      if (!isValidConnection(connection)) return;
      const source = nodes.find((node) => node.id === connection.source);
      const pin = source?.data.outputs?.find(
        (item) => item.id === connection.sourceHandle,
      );
      if (!pin) return;
      setEdges((current) =>
        addEdge(
          {
            ...connection,
            id: crypto.randomUUID(),
            kind: pin.kind,
            ...baseEdgeVisuals(pin),
          },
          current,
        ),
      );
      setDirty(true);
    },
    [isValidConnection, nodes],
  );
  const onReconnectStart = useCallback(
    (
      event: React.MouseEvent,
      edge: FlowEdge,
      stableHandle: "source" | "target",
    ) => {
      // React Flow reports the stationary end. Moving the target/input pin means
      // its source is stationary, so only that direction supports Ctrl-drag.
      reconnectRef.current = {
        edgeID: edge.id,
        allowed: stableHandle === "source" && (event.ctrlKey || event.metaKey),
        completed: false,
      };
      setConnecting(undefined);
    },
    [],
  );
  const onReconnect = useCallback(
    (edge: FlowEdge, connection: Connection) => {
      const reconnecting = reconnectRef.current;
      if (
        !reconnecting?.allowed ||
        reconnecting.edgeID !== edge.id ||
        !isValidConnection(connection)
      )
        return;
      reconnecting.completed = true;
      setEdges((current) =>
        reconnectEdge(edge, connection, current, { shouldReplaceId: false }),
      );
      setDirty(true);
    },
    [isValidConnection],
  );
  const onReconnectEnd = useCallback(
    (
      _event: MouseEvent | TouchEvent,
      edge: FlowEdge,
      _stableHandle: "source" | "target",
      connectionState: { isValid: boolean | null },
    ) => {
      const reconnecting = reconnectRef.current;
      if (!reconnecting || reconnecting.edgeID !== edge.id) return;
      if (
        reconnecting.allowed &&
        !reconnecting.completed &&
        connectionState.isValid !== true
      ) {
        setEdges((current) => current.filter((item) => item.id !== edge.id));
        setDirty(true);
      }
      reconnectRef.current = undefined;
    },
    [],
  );
  const addNode = useCallback(
    (definition: NodeDefinition, position = { x: 280, y: 180 }) => {
      const node = createEditorNode(definition, snapPosition(position));
      setNodes((current) => [...current, node]);
                if (backendResolvedNodeTypes.has(node.data.type)) {
                        void desktop.resolveNodeDefinition({
                                id: node.id,
                                type: node.data.type,
                                position: node.position,
                                data: { config: node.data.config },
                        }).then((raw) => {
                                const resolved = localizeNodeDefinitions([raw], i18n.language)[0] ?? raw;
                                setNodes((current) => current.map((item) => item.id === node.id ? {
                                        ...item,
                                        data: { ...item.data, resolvedDefinition: resolved, inputs: resolveInputs(resolved, item.data.config), outputs: resolveOutputs(resolved, item.data.config) },
                                } : item));
                        }).catch((reason: unknown) => setError(reason instanceof Error ? reason.message : t("editor.saveFailed")));
                }
      setSelectedID(node.id);
      setDirty(true);
      return node.id;
    },
    [snapPosition],
  );
  const save = async () => {
    if (!pipeline) return;
    try {
      setBusy("save");
      const draftDefinition = {
        schemaVersion: pipeline.draftDefinition.schemaVersion,
        nodes: dehydrateNodes(nodes),
        edges,
        viewport: flow?.getViewport() ?? pipeline.draftDefinition.viewport,
      };
      const saved = await desktop.savePipeline({
        ...pipeline,
        draftDefinition,
      });
      setPipeline(saved);
      setDirty(false);
      await onRefresh();
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : t("editor.saveFailed"),
      );
    } finally {
      setBusy("");
    }
  };
  const beginNameEdit = () => {
    if (!pipeline || busy !== "") return;
    setNameDraft(pipeline.name);
    setEditingName(true);
  };
  const cancelNameEdit = () => setEditingName(false);
  const commitNameEdit = async () => {
    if (!pipeline || !editingName) return;
    const name = nameDraft.trim();
    if (!name || name === pipeline.name) {
      setEditingName(false);
      return;
    }
    try {
      setBusy("rename");
      const draftDefinition = {
        schemaVersion: pipeline.draftDefinition.schemaVersion,
        nodes: dehydrateNodes(nodes),
        edges,
        viewport: flow?.getViewport() ?? pipeline.draftDefinition.viewport,
      };
      const saved = await desktop.savePipeline({
        ...pipeline,
        name,
        draftDefinition,
      });
      setPipeline(saved);
      setDirty(false);
      setEditingName(false);
      await onRefresh();
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : t("editor.renameFailed"),
      );
    } finally {
      setBusy("");
    }
  };
  const updatePipelineAppearance = async (
    change: Partial<Pick<Pipeline, "icon" | "iconColor" | "iconBackground">>,
  ) => {
    if (!pipeline || busy !== "" || Object.entries(change).every(([key, value]) => pipeline[key as keyof Pipeline] === value)) return;
    try {
      setBusy("appearance");
      const draftDefinition = {
        schemaVersion: pipeline.draftDefinition.schemaVersion,
        nodes: dehydrateNodes(nodes),
        edges,
        viewport: flow?.getViewport() ?? pipeline.draftDefinition.viewport,
      };
      const saved = await desktop.savePipeline({
        ...pipeline,
        ...change,
        draftDefinition,
      });
      setPipeline(saved);
      setDirty(false);
      await onRefresh();
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : t("editor.appearanceFailed"),
      );
    } finally {
      setBusy("");
    }
  };
  const publish = async () => {
    if (!pipeline) return;
    try {
      setBusy("publish");
      const draftDefinition = {
        schemaVersion: pipeline.draftDefinition.schemaVersion,
        nodes: dehydrateNodes(nodes),
        edges,
        viewport: flow?.getViewport() ?? pipeline.draftDefinition.viewport,
      };
      const saved = await desktop.savePipeline({
        ...pipeline,
        draftDefinition,
      });
      const published = await desktop.publishPipeline(saved);
      setPipeline(published);
      setDirty(false);
      await onRefresh();
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : t("editor.publishFailed"),
      );
    } finally {
      setBusy("");
    }
  };
  const stop = async () => {
    if (!pipeline) return;
    try {
      setBusy("stop");
      await desktop.cancelPipelineExecution(pipeline.id);
      await onRefresh();
      try {
        const nextHistory = asArray(await desktop.listExecutions(pipeline.id));
        setHistory(nextHistory);
        applyExecution(nextHistory[0]);
        setEdges((current) => withExecutionEdges(current, nextHistory[0]));
      } catch {
        /* refresh history in background */
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("editor.runFailed"));
    } finally {
      setBusy("");
    }
  };
  const run = async () => {
    if (!pipeline) return;
    const trigger =
      nodes.find((node) => node.data.type === "trigger:button") ??
      nodes.find((node) => node.data.type.startsWith("trigger:"));
    if (!trigger) {
      setError(t("editor.addTrigger"));
      return;
    }
    try {
      setBusy("run");
      setRunLogKey((current) => current + 1);
      const draftDefinition = {
        schemaVersion: pipeline.draftDefinition.schemaVersion,
        nodes: dehydrateNodes(nodes),
        edges,
        viewport: flow?.getViewport() ?? pipeline.draftDefinition.viewport,
      };
      const saved = await desktop.savePipeline({
        ...pipeline,
        draftDefinition,
      });
      setPipeline(saved);
      setDirty(false);
      const execution = await desktop.runPipelineDraft(saved.id, trigger.id);
      applyExecution(execution);
      setEdges((current) => withExecutionEdges(current, execution));
      setHistory((current) => [
        execution,
        ...current.filter((item) => item.id !== execution.id),
      ]);
      await onRefresh();
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : t("editor.runFailed"),
      );
    } finally {
      try {
        const nextHistory = asArray(await desktop.listExecutions(pipelineID));
        setHistory(nextHistory);
        applyExecution(nextHistory[0]);
        setEdges((current) => withExecutionEdges(current, nextHistory[0]));
      } catch {
        /* The run result remains visible when refreshing history is unavailable. */
      }
      setBusy("");
    }
  };
  const updateConfigValues = (values: Record<string, unknown>) => {
    if (!selectedID) return;
    const selectedNode = nodes.find((node) => node.id === selectedID);
    // Choosing an agent chat mode supersedes the legacy history toggle so a
    // stale saved flag cannot resurrect chat-history behaviour.
    if (
      values.chatMode !== undefined &&
      (selectedNode?.data.type === "llm:agent" || selectedNode?.data.type === "llm:coding_agent")
    ) {
      const { pullChatHistory: _legacy, ...rest } = values;
      values = rest;
      if (selectedNode.data.config.pullChatHistory !== undefined) {
        values = { pullChatHistory: undefined, ...values };
      }
    }
    const nextConfig = selectedNode
      ? normalizeNodeConfig(selectedNode.data.type, {
          ...selectedNode.data.config,
          ...values,
        })
      : undefined;
        // The backend resolves Twitch event ports from the selected EventSub type.
        // Keep existing wires until that authoritative resolution finishes; this
        // prevents a field edit from silently deleting graph data edges.
    const removedOutputIDs = nextConfig && selectedNode?.data.type !== "twitch:event"
      ? (selectedNode!.data.outputs ?? [])
          .filter((output) =>
            !resolveOutputs(selectedDefinition, nextConfig).some(
              (next) => next.id === output.id,
            ),
          )
          .map((output) => output.id)
      : [];
        // Toggle-driven input pins (agent chat history) drop their wires the
        // same way removed outputs do, so no edge targets a vanished handle.
    const removedInputIDs = nextConfig && selectedNode
      ? (selectedNode.data.inputs ?? [])
          .filter((input) =>
            !resolveInputs(selectedDefinition, nextConfig).some(
              (next) => next.id === input.id,
            ),
          )
          .map((input) => input.id)
      : [];
    setNodes((current) =>
      current.map((node) => {
        if (node.id !== selectedID) return node;
        const config = normalizeNodeConfig(node.data.type, {
          ...node.data.config,
          ...values,
        });
        return {
          ...node,
          data: {
            ...node.data,
            config,
            inputs: resolveInputs(selectedDefinition, config),
            outputs: resolveOutputs(selectedDefinition, config),
          },
        };
      }),
    );
                // Nodes with backend-owned dynamic contracts re-resolve after every config
                // change so the canvas, compatibility highlighting, and runtime share one
                // port shape. Twitch picks ports by event type; global variables pick the
                // global-variable pins by the selected declaration and operation.
                if (selectedNode && backendResolvedNodeTypes.has(selectedNode.data.type) && nextConfig) {
                void desktop.resolveNodeDefinition({
                        id: selectedNode.id,
                        type: selectedNode.data.type,
                        position: selectedNode.position,
                        data: { config: nextConfig },
                }).then((raw) => {
                        const resolved = localizeNodeDefinitions([raw], i18n.language)[0] ?? raw;
                        setNodes((current) => current.map((node) => node.id === selectedNode.id ? {
                                ...node,
                        data: { ...node.data, resolvedDefinition: resolved, inputs: resolveInputs(resolved, nextConfig), outputs: resolveOutputs(resolved, nextConfig) },
                        } : node));
                }).catch((reason: unknown) => setError(reason instanceof Error ? reason.message : t("editor.saveFailed")));
        }
    if (removedOutputIDs.length > 0 || removedInputIDs.length > 0) {
      setEdges((current) =>
        current.filter(
          (edge) =>
            edge.source !== selectedID ||
            !removedOutputIDs.includes(edge.sourceHandle ?? "out"),
        ).filter(
          (edge) =>
            edge.target !== selectedID ||
            !removedInputIDs.includes(edge.targetHandle ?? "in"),
        ),
      );
    }
    setDirty(true);
  };
  const updateConfig = (field: ConfigField, value: unknown) => {
    updateConfigValues({ [field.name]: parseFieldValue(field, value) });
  };
  const removeSelected = () => {
    const selectedNodeIDs = nodes.filter((node) => node.selected).map((node) => node.id);
    const removedIDs = new Set(selectedNodeIDs.length > 0 ? selectedNodeIDs : selectedID ? [selectedID] : []);
    if (removedIDs.size === 0) return;
    setNodes((current) => current.filter((node) => !removedIDs.has(node.id)));
    setEdges((current) =>
      current.filter(
        (edge) => !removedIDs.has(edge.source) && !removedIDs.has(edge.target),
      ),
    );
    setSelectedID(undefined);
    setDirty(true);
  };
  const duplicateSelected = () => {
    if (!selected) return;
    const duplicate = {
      ...selected,
      id: `${selected.id}-copy`,
      position: snapPosition({
        x: selected.position.x + 36,
        y: selected.position.y + 36,
      }),
      selected: true,
    };
    setNodes((current) => [
      ...current.map((node) => ({ ...node, selected: false })),
      duplicate,
    ]);
    setSelectedID(duplicate.id);
    setDirty(true);
  };
  const autoLayout = () => {
    setNodes((current) =>
      current.map((node, index) => ({
        ...node,
        position: {
          x: 110 + (index % 4) * 270,
          y: 130 + Math.floor(index / 4) * 190,
        },
      })),
    );
    setDirty(true);
  };
  const drop = (event: React.DragEvent) => {
    event.preventDefault();
    const nodeType = event.dataTransfer.getData("application/neuropipe-node");
    const definition = definitions.find((item) => item.type === nodeType);
    if (definition && flow)
      addNode(
        definition,
        flow.screenToFlowPosition(
          { x: event.clientX, y: event.clientY },
          { snapToGrid, snapGrid: editorSnapGrid },
        ),
      );
  };
  const openCanvasMenu = (
    event: { clientX: number; clientY: number; preventDefault: () => void },
    nodeID?: string,
    connection?: { source: string; sourceHandle: string | null },
    edgeID?: string,
    edgeKind?: string,
  ) => {
    event.preventDefault();
    const position = flow?.screenToFlowPosition({
      x: event.clientX,
      y: event.clientY,
    }) ?? { x: event.clientX, y: event.clientY };
    const bounds = canvasRef.current?.getBoundingClientRect();
    const menuWidth = 320;
    const menuHeight = edgeID ? 92 : 468;
    const menuPosition = contextMenuPosition(
      event,
      { width: menuWidth, height: menuHeight },
      bounds,
    );
    setMenu({
      x: menuPosition.x,
      y: menuPosition.y,
      position,
      nodeID,
      edgeID,
      edgeKind,
      source: connection?.source,
      sourceHandle: connection?.sourceHandle,
    });
    setMenuSearch("");
  };
  const removeEdge = useCallback((edgeID: string) => {
    setEdges((current) => current.filter((edge) => edge.id !== edgeID));
    setDirty(true);
  }, []);
  // Inserting a reroute splits the wire with a compact reroute node: the
  // node's pins behave exactly like node pins, so the reroute can later fan
  // out to several targets. Graphs saved with legacy wire waypoints keep
  // rendering those as before.
  const insertReroute = useCallback(
    (edgeID: string, position: { x: number; y: number }) => {
      const edge = edges.find((item) => item.id === edgeID);
      if (!edge || !edge.source || !edge.target) return;
      if (edge.kind === "tool") return;
      const definition = definitions.find(
        (item) => item.type === (edge.kind === "exec" ? "flow:reroute" : "data:reroute"),
      );
      const rerouteInput = definition?.inputs.find((pin) => pin.kind === (edge.kind === "exec" ? "exec" : "data"));
      const rerouteOutput = definition?.outputs.find((pin) => pin.kind === (edge.kind === "exec" ? "exec" : "data"));
      if (!definition || !rerouteInput || !rerouteOutput) return;
      const rerouteID = addNode(definition, position);
      const sourcePin = nodes
        .find((node) => node.id === edge.source)
        ?.data.outputs?.find((pin) => pin.id === edge.sourceHandle);
      setEdges((current) => {
        const existing = current.find((item) => item.id === edgeID);
        if (!existing) return current;
        return [
          ...current.filter((item) => item.id !== edgeID),
          {
            id: crypto.randomUUID(),
            source: existing.source,
            sourceHandle: existing.sourceHandle,
            target: rerouteID,
            targetHandle: rerouteInput.id,
            kind: existing.kind,
            ...baseEdgeVisuals(sourcePin),
          },
          {
            id: crypto.randomUUID(),
            source: rerouteID,
            sourceHandle: rerouteOutput.id,
            target: existing.target,
            targetHandle: existing.targetHandle,
            kind: existing.kind,
            ...baseEdgeVisuals(rerouteOutput),
          },
        ];
      });
      setDirty(true);
    },
    [addNode, definitions, edges, nodes],
  );
  const addFromMenu = (definition: NodeDefinition) => {
    if (!menu) return;
    const id = addNode(definition, menu.position);
    if (menu.source) {
      const source = nodes.find((node) => node.id === menu.source);
      const pin = source?.data.outputs?.find(
        (item) => item.id === menu.sourceHandle,
      );
      const target =
        pin && definition.inputs.find((input) => compatiblePins(pin, input));
      if (pin && target) {
        setEdges((current) => [
          ...current,
          {
            id: crypto.randomUUID(),
            source: menu.source!,
            sourceHandle: pin.id,
            target: id,
            targetHandle: target.id,
            kind: pin.kind,
            ...baseEdgeVisuals(pin),
          },
        ]);
        setDirty(true);
      }
    }
    setMenu(undefined);
    setConnecting(undefined);
  };
  const menuDefinitions = useMemo(() => {
    const source = menu?.source
      ? nodes.find((node) => node.id === menu.source)
      : undefined;
    const sourcePin = source?.data.outputs?.find(
      (pin) => pin.id === menu?.sourceHandle,
    );
    // The menu filters the full catalog: the node library's search box must
    // not narrow what the canvas context menu offers.
    return definitions.filter(
      (definition) =>
        !contextOnlyNodeTypes.has(definition.type) &&
        `${definition.label} ${definition.category} ${definition.description}`
          .toLowerCase()
          .includes(menuSearch.toLowerCase()) &&
        (!sourcePin ||
          definition.inputs.some((input) => compatiblePins(sourcePin, input))),
    );
  }, [
    definitions,
    menu?.source,
    menu?.sourceHandle,
    menuSearch,
    nodes,
  ]);

  if (!pipeline || busy === "load")
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-500">
        <Loader2 className="mr-2 size-4 animate-spin" />
        {t("editor.loading")}
      </div>
    );
  if (pipeline.draftDefinition.schemaVersion !== 3)
    return (
      <section className="flex h-full items-center justify-center p-8">
        <div className="surface max-w-lg rounded-xl p-6">
          <p className="text-sm font-semibold">{t("editor.legacyTitle")}</p>
          <p className="mt-2 text-sm leading-6 text-zinc-500">
            {t("editor.legacyDescription")}
          </p>
          <Button className="mt-5" onClick={() => setScreen("pipelines")}>
            <ArrowLeft className="size-4" />
            {t("editor.back")}
          </Button>
        </div>
      </section>
    );
  return (
    <section className="flex h-full min-h-0 flex-col">
      <header className="title-drag flex h-16 shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-950 px-5">
        <div className="title-no-drag flex min-w-0 items-center gap-3">
          <Button
            size="sm"
            variant="ghost"
            aria-label={t("editor.back")}
            onClick={() => setScreen("pipelines")}
          >
            <ArrowLeft className="size-4" />
          </Button>
          <LucideIconPicker
            value={pipeline.icon}
            label={t("editor.pipelineIcon")}
            disabled={busy !== ""}
            iconColor={pipeline.iconColor}
            iconBackground={pipeline.iconBackground}
            onValueChange={(icon) => void updatePipelineAppearance({ icon })}
          />
          <IconAppearancePicker
            iconColor={pipeline.iconColor}
            iconBackground={pipeline.iconBackground}
            disabled={busy !== ""}
            onIconColorChange={(iconColor) => void updatePipelineAppearance({ iconColor })}
            onIconBackgroundChange={(iconBackground) => void updatePipelineAppearance({ iconBackground })}
          />
          <div className="min-w-0">
            {editingName ? (
              <Input
                autoFocus
                aria-label={t("editor.pipelineName")}
                className="h-7 w-64 max-w-full text-sm font-semibold"
                value={nameDraft}
                disabled={busy === "rename"}
                onChange={(event) => setNameDraft(event.target.value)}
                onBlur={() => void commitNameEdit()}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    void commitNameEdit();
                  } else if (event.key === "Escape") {
                    event.preventDefault();
                    cancelNameEdit();
                  }
                }}
              />
            ) : (
              <Tooltip content={t("editor.rename")} side="bottom" align="start">
                <button
                  type="button"
                  className="block max-w-full truncate text-left text-sm font-semibold hover:text-zinc-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/40"
                  onClick={beginNameEdit}
                >
                  {pipeline.name}
                </button>
              </Tooltip>
            )}
            <p className="text-xs text-zinc-600">
              {pipeline.hasUnpublishedChanges
                ? t("pipelineStatus.draftUpdate", { version: pipeline.publishedRevision })
                : pipeline.status === "active"
                ? t("editor.published", { version: pipeline.publishedRevision })
                : t("editor.draft")}
              {dirty ? ` · ${t("editor.unsaved")}` : ""}
            </p>
          </div>
        </div>
        <div className="title-no-drag flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={() => void save()}
            disabled={!dirty || busy !== ""}
          >
            {busy === "save" ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <Save className="size-3.5" />
            )}
            {t("editor.save")}
          </Button>
          <Tooltip content={t("editor.runTitle")} side="bottom" size="body" className="max-w-72 px-3 py-2 text-zinc-300">
            <Button size="sm" variant="outline" onClick={() => void run()} disabled={busy !== "" || asArray(history)[0]?.status === "running"}>
              {busy === "run" ? <Loader2 className="size-3.5 animate-spin" /> : <Play className="size-3.5" />}
              {t("editor.runDraft")}
            </Button>
          </Tooltip>
          {asArray(history)[0]?.status === "running" ? (
            <Button size="sm" variant="danger" onClick={() => void stop()} disabled={busy !== "" && busy !== "stop"}>
              {busy === "stop" ? <Loader2 className="size-3.5 animate-spin" /> : <Square className="size-3.5" />}
              {t("pipelines.stop")}
            </Button>
          ) : null}
          <Button
            size="sm"
            onClick={() => void publish()}
            disabled={busy !== ""}
          >
            {busy === "publish" ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <UploadCloud className="size-3.5" />
            )}
            {t("editor.publish")}
          </Button>
        </div>
      </header>
      <div className="min-h-0 flex-1">
        <div
          className="grid h-full min-h-0"
          style={{
            gridTemplateColumns: `${showPalette ? "246px " : ""}minmax(0,1fr)${showInspector ? " 300px" : ""}`,
          }}
        >
          {showPalette && (
            <BlueprintNodeLibrary
              definitions={filteredDefinitions}
              search={search}
              onSearch={setSearch}
              onAdd={addNode}
              dragMime="application/neuropipe-node"
              preferenceKey={nodeLibraryCollapsedCategoriesKey}
            />
          )}
          <div ref={canvasRef} className="relative min-w-0">
            <ReactFlow
              className="select-none"
              nodes={nodes}
              edges={edges.map((edge) =>
                edge.waypoints && edge.waypoints.length > 0
                  ? { ...edge, type: "waypoint", data: { waypoints: edge.waypoints } }
                  : { ...edge },
              )}
              edgeTypes={edgeTypes}
              nodeTypes={nodeTypes}
              defaultEdgeOptions={defaultEdgeOptions}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              isValidConnection={isValidConnection}
              onConnect={(connection) => {
                onConnect(connection);
                setConnecting(undefined);
              }}
              onConnectStart={(_, parameters) => {
                if (reconnectRef.current || !parameters.nodeId) return;
                setConnecting({
                  source: parameters.nodeId,
                  sourceHandle: parameters.handleId,
                });
              }}
              onConnectEnd={(event) => {
                if (reconnectRef.current) return;
                const target = event.target as Element;
                if (
                  connecting &&
                  target.classList.contains("react-flow__pane") &&
                  "clientX" in event
                )
                  openCanvasMenu(event, undefined, connecting);
              }}
              onReconnectStart={onReconnectStart}
              onReconnect={onReconnect}
              onReconnectEnd={onReconnectEnd}
              onNodeClick={(_, node) => setSelectedID(node.id)}
              onNodeContextMenu={(event, node) => {
                if (!nodes.find((item) => item.id === node.id)?.selected) {
                  setNodes((current) => current.map((item) => ({ ...item, selected: item.id === node.id })));
                }
                setSelectedID(node.id);
                openCanvasMenu(event, node.id);
              }}
              onSelectionContextMenu={(event, selectedNodes) => {
                const nodeID = selectedNodes[0]?.id;
                if (!nodeID) return;
                setSelectedID(nodeID);
                openCanvasMenu(event, nodeID);
              }}
              onEdgeContextMenu={(event, edge) =>
                openCanvasMenu(event, undefined, undefined, edge.id, edge.kind)
              }
              onPaneClick={() => {
                setSelectedID(undefined);
                setMenu(undefined);
              }}
              onPaneContextMenu={(event) => openCanvasMenu(event)}
              multiSelectionKeyCode={["Shift", "Control", "Meta"]}
              onInit={setFlow}
              onDrop={drop}
              onDragOver={(event) => {
                event.preventDefault();
                event.dataTransfer.dropEffect = "move";
              }}
              snapToGrid={snapToGrid}
              snapGrid={editorSnapGrid}
              fitView
            >
              <Background color="#27272a" gap={20} size={1} />
              <Controls className="blueprint-controls" showInteractive={false} />
              <Panel
                position="top-left"
                className="!m-3 rounded-md border border-zinc-700 bg-zinc-950 p-1"
              >
                <Tooltip content={t("editor.library")} side="bottom" align="start">
                  <Button size="sm" variant={showPalette ? "secondary" : "ghost"} className="size-7 p-0" onClick={() => setShowPalette((value) => !value)} aria-label={t("editor.library")} aria-pressed={showPalette}>
                    <PanelLeft className="size-3.5" />
                  </Button>
                </Tooltip>
              </Panel>
              <Panel
                position="top-right"
                className="!m-3 rounded-md border border-zinc-700 bg-zinc-950 p-1"
              >
                <Tooltip content={t("editorActions.inspector")} side="bottom" align="end">
                  <Button size="sm" variant={showInspector ? "secondary" : "ghost"} className="size-7 p-0" onClick={() => setShowInspector((value) => !value)} aria-label={t("editorActions.inspector")} aria-pressed={showInspector}>
                    <PanelRight className="size-3.5" />
                  </Button>
                </Tooltip>
              </Panel>
              <Panel
                position="bottom-right"
                className="!m-3 flex gap-1 rounded-md border border-zinc-700 bg-zinc-950 p-1"
              >
                <Tooltip content={t("editor.layout")} side="top">
                  <Button size="sm" variant="ghost" className="size-7 p-0" onClick={autoLayout} aria-label={t("editor.layout")}>
                    <LayoutGrid className="size-3.5" />
                  </Button>
                </Tooltip>
                <Tooltip content={snapToGrid ? t("editor.snapOff") : t("editor.snapOn")} side="top">
                  <Button size="sm" variant={snapToGrid ? "secondary" : "ghost"} className="size-7 p-0" aria-label={snapToGrid ? t("editor.snapOff") : t("editor.snapOn")} aria-pressed={snapToGrid} onClick={() => setGridSnapMode(snapToGrid ? "off" : "on")}>
                    <Magnet className="size-3.5" />
                  </Button>
                </Tooltip>
                <Tooltip content={t("editorActions.duplicate")} side="top">
                  <Button
                    size="sm"
                    variant="ghost"
                    className="size-7 p-0"
                    aria-label={t("editorActions.duplicate")}
                    onClick={duplicateSelected}
                    disabled={!selected}
                  >
                    <Copy className="size-3.5" />
                  </Button>
                </Tooltip>
                <Tooltip content={t("editorActions.delete")} side="top">
                  <Button size="sm" variant="ghost" className="size-7 p-0" onClick={removeSelected} disabled={!selected} aria-label={t("editorActions.delete")}>
                    <Trash2 className="size-3.5 text-red-300" />
                  </Button>
                </Tooltip>
              </Panel>
            </ReactFlow>
            {menu && (
              <BlueprintContextMenu
                menu={menu}
                definitions={menuDefinitions}
                search={menuSearch}
                onSearch={setMenuSearch}
                onAdd={addFromMenu}
                onDuplicate={duplicateSelected}
                onDelete={removeSelected}
                onRemoveEdge={removeEdge}
                onInsertReroute={insertReroute}
                preferenceKey={nodeLibraryCollapsedCategoriesKey}
                onClose={() => {
                  setMenu(undefined);
                  setConnecting(undefined);
                }}
              />
            )}
          </div>
          {showInspector ? <Inspector
            node={selected}
            definition={selectedDefinition}
            sourceFields={selectedSourceFields}
            onUpdate={updateConfig}
            onUpdateConfig={updateConfigValues}
            history={history}
            runLogKey={runLogKey}
          /> : null}
        </div>
      </div>
    </section>
  );
}

function statusTextClass(status: string) {
  if (status === "completed") return "text-emerald-300";
  if (status === "failed" || status === "cancelled") return "text-red-300";
  return "text-amber-300";
}

function statusDotClass(status: string) {
  if (status === "completed") return "bg-emerald-400";
  if (status === "failed" || status === "cancelled") return "bg-red-400";
  return "bg-amber-400";
}

function formatExecutionValue(value: unknown) {
  if (value === undefined) return i18n.t("editor.noData");
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function NodeRunDetails({ nodeRun }: { nodeRun: NodeRun }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(nodeRun.status === "failed");
  useEffect(
    () => setExpanded(nodeRun.status === "failed"),
    [nodeRun.nodeId, nodeRun.startedAt, nodeRun.status],
  );
  return (
    <div className="border-t border-zinc-800 first:border-t-0">
      <button
        type="button"
        className="flex w-full items-center gap-2 px-3 py-2.5 text-left hover:bg-zinc-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-600"
        aria-expanded={expanded}
        onClick={() => setExpanded((current) => !current)}
      >
        <ChevronRight
          className={cn(
            "size-3.5 shrink-0 text-zinc-600 transition-transform",
            expanded && "rotate-90",
          )}
        />
        <span
          className={cn(
            "size-1.5 shrink-0 rounded-full",
            statusDotClass(nodeRun.status),
          )}
        />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-xs font-medium text-zinc-300">
            {nodeRun.nodeId}
          </span>
          <span className="block truncate font-mono text-[10px] text-zinc-600">
            {nodeRun.nodeType}
          </span>
        </span>
        <span
          className={cn(
            "text-[10px] font-medium",
            statusTextClass(nodeRun.status),
          )}
        >
          {nodeRun.status}
        </span>
      </button>
      {expanded ? (
        <div className="space-y-2 border-t border-zinc-800 bg-zinc-950/60 p-3">
          <div className="grid gap-2">
            <ExecutionData title={t("editor.input")} value={nodeRun.input} />
            {nodeRun.error ? (
              <div className="rounded-md border border-red-500/30 bg-red-500/10 p-2.5">
                <p className="text-[10px] font-semibold uppercase tracking-[.12em] text-red-300">
                  {t("editorActions.error")}
                </p>
                <pre className="muted-scroll mt-2 max-h-32 overflow-auto whitespace-pre-wrap break-words font-mono text-[11px] leading-4 text-red-200">
                  {nodeRun.error}
                </pre>
              </div>
            ) : (
              <ExecutionData title={t("editor.output")} value={nodeRun.output} />
            )}
          </div>
        </div>
      ) : null}
    </div>
  );
}

function ExecutionData({ title, value }: { title: string; value: unknown }) {
  return (
    <div className="overflow-hidden rounded-md border border-zinc-800 bg-zinc-950">
      <p className="border-b border-zinc-800 bg-zinc-900/70 px-2.5 py-1.5 text-[10px] font-semibold uppercase tracking-[.12em] text-zinc-500">
        {title}
      </p>
      <pre className="muted-scroll max-h-40 overflow-auto whitespace-pre-wrap break-words p-2.5 font-mono text-[11px] leading-4 text-zinc-300">
        {formatExecutionValue(value)}
      </pre>
    </div>
  );
}

function ExecutionLog({ runs }: { runs: Execution[] }) {
  const { t } = useTranslation();
  if (runs.length === 0)
    return (
      <div className="p-6 text-center text-xs leading-5 text-zinc-600">
        <Play className="mx-auto mb-2 size-4" />
        {t("editorActions.runToInspect")}
      </div>
    );
  return (
    <div className="divide-y divide-zinc-800">
      {runs.map((run) => (
        <article key={run.id} className="bg-zinc-950/30">
          <div className="flex items-start justify-between gap-3 px-3 py-3">
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span
                  className={cn(
                    "size-1.5 shrink-0 rounded-full",
                    statusDotClass(run.status),
                  )}
                />
                <p
                  className={cn(
                    "text-xs font-medium",
                    statusTextClass(run.status),
                  )}
                >
                  {run.status}
                </p>
                <span className="text-[10px] text-zinc-600">
                  {t("editorActions.nodes", { count: asArray(run.nodeRuns).length })}
                </span>
              </div>
              <p className="mt-1 font-mono text-[10px] text-zinc-600">
                {formatDate(run.startedAt)} ·{" "}
                {run.triggerId?.startsWith("draft:")
                  ? t("editorActions.draftRun")
                  : t("editorActions.publishedTrigger")}
              </p>
            </div>
          </div>
          {run.error ? (
            <p className="mx-3 mb-3 rounded-md border border-red-500/30 bg-red-500/10 p-2 text-[11px] leading-4 text-red-200">
              {run.error}
            </p>
          ) : null}
          {asArray(run.nodeRuns).length > 0 ? (
            <div className="border-t border-zinc-800">
              <p className="px-3 py-2 text-[10px] font-semibold uppercase tracking-[.12em] text-zinc-600">
                {t("editorActions.nodeResults")}
              </p>
              {asArray(run.nodeRuns).map((nodeRun) => (
                <NodeRunDetails
                  key={`${run.id}-${nodeRun.nodeId}-${nodeRun.startedAt}`}
                  nodeRun={nodeRun}
                />
              ))}
            </div>
          ) : (
            <p className="border-t border-zinc-800 px-3 py-3 text-[11px] text-zinc-600">
              {t("editorActions.noNodeResults")}
            </p>
          )}
        </article>
      ))}
    </div>
  );
}

function Inspector({
  node,
  definition,
  sourceFields,
  onUpdate,
  onUpdateConfig,
  history,
  runLogKey,
}: {
  node?: EditorNode;
  definition?: NodeDefinition;
  sourceFields: DataField[];
  onUpdate: (field: ConfigField, value: unknown) => void;
  onUpdateConfig: (values: Record<string, unknown>) => void;
  history: Execution[];
  runLogKey: number;
}) {
  const { t } = useTranslation();
  const openDocumentation = useUIStore((state) => state.openDocumentation);
  const [tab, setTab] = useState<"config" | "runs">("config");
  const [secretOptions, setSecretOptions] = useState<
    { value: string; label: string }[]
  >([]);
  const [documentation, setDocumentation] = useState<DocumentationReference>();
  const [documentationMessage, setDocumentationMessage] = useState(() => t("editor.docsUnavailable"));
  const runs = asArray(history);
  const fields = asArray(definition?.fields).filter((field) => {
    if (!field.visibleWhen) return true;
    const value =
      node?.data.config[field.visibleWhen] ??
      definition?.defaultConfig?.[field.visibleWhen];
    if (field.visibleWhen === "chatMode") return value === "history";
    return value === true || value === "true";
  });
  const capabilities = asArray(definition?.capabilities);
  useEffect(() => {
    if (runLogKey > 0) setTab("runs");
  }, [runLogKey]);
  useEffect(() => {
    let cancelled = false;
    void desktop
      .listSecrets()
      .then((secrets) => {
        if (cancelled) return;
        setSecretOptions(
          asArray(secrets).map((secret) => ({
            value: secret.name,
            label: secret.name,
          })),
        );
      })
      .catch(() => {
        if (!cancelled) setSecretOptions([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);
  useEffect(() => {
    if (!definition) {
      setDocumentation(undefined);
      return;
    }
    let cancelled = false;
    void desktop.getDocumentationForNode(definition.type).then((reference) => {
      if (!cancelled) {
        setDocumentation(reference);
        setDocumentationMessage("");
      }
    }).catch((reason) => {
      if (!cancelled) {
        setDocumentation(undefined);
        setDocumentationMessage(reason instanceof Error ? reason.message : t("editor.docsUnavailable"));
      }
    });
    return () => { cancelled = true; };
  }, [definition?.type, t]);
  return (
    <aside className="muted-scroll min-h-0 overflow-y-auto border-l border-zinc-800 bg-zinc-950">
      <div className="flex border-b border-zinc-800">
        <button
          onClick={() => setTab("config")}
          className={cn(
            "flex-1 px-3 py-3 text-xs font-medium",
            tab === "config"
              ? "border-b-2 border-white text-zinc-100"
              : "text-zinc-600",
          )}
        >
          {t("editorActions.inspector")}
        </button>
        <button
          onClick={() => setTab("runs")}
          className={cn(
            "flex-1 px-3 py-3 text-xs font-medium",
            tab === "runs"
              ? "border-b-2 border-white text-zinc-100"
              : "text-zinc-600",
          )}
        >
          {t("editorActions.executionLog")}
        </button>
      </div>
      {tab === "runs" ? (
        <ExecutionLog runs={runs} />
      ) : !node || !definition ? (
        <div className="p-5 text-center text-xs leading-5 text-zinc-600">
          <MousePointer2 className="mx-auto mb-2 size-4" />
          {t("editorActions.selectNode")}
        </div>
      ) : (
        <div className="p-4">
          <div className="mb-5">
            <div className="flex items-center justify-between gap-2">
              <p className="text-sm font-medium">{definition.label}</p>
              <Tooltip content={documentationMessage || t("editor.openDocs")} side="left">
                <Button size="sm" variant="ghost" disabled={!documentation} onClick={() => documentation && openDocumentation(documentation.documentId, documentation.anchor)}>
                  <BookOpen className="size-3.5" />
                  {t("editorActions.docs")}
                </Button>
              </Tooltip>
            </div>
            <p className="mt-1 text-xs leading-5 text-zinc-500">
              {definition.description}
            </p>
          </div>
          {node.data.lastRun ? (
            <div className="mb-5 overflow-hidden rounded-md border border-zinc-800">
              <p className="border-b border-zinc-800 bg-zinc-900/70 px-3 py-2 text-[10px] font-semibold uppercase tracking-[.12em] text-zinc-500">
                {t("editorActions.latestResult")}
              </p>
              <NodeRunDetails nodeRun={node.data.lastRun} />
            </div>
          ) : null}
          <KnownOutputFields pins={asArray(node.data.outputs)} />
          <div className="space-y-4">
            {fields.map((field) => (
              <ConfigControl
                key={field.name}
                field={field}
                value={node.data.config[field.name]}
                defaultValue={definition.defaultConfig?.[field.name]}
                secretOptions={secretOptions}
                sourceFields={sourceFields}
                nodeConfig={node.data.config}
                onChange={(value) => onUpdate(field, value)}
                onConfigChange={onUpdateConfig}
              />
            ))}
          </div>
          {capabilities.length > 0 && (
            <div className="mt-6 border-t border-zinc-800 pt-4">
              <p className="text-[10px] font-semibold uppercase tracking-[.14em] text-zinc-600">
                {t("editorActions.capabilities")}
              </p>
              <div className="mt-2 flex flex-wrap gap-1">
                {capabilities.map((capability) => (
                  <span
                    key={capability}
                    className="rounded bg-zinc-800 px-1.5 py-1 text-[10px] text-zinc-400"
                  >
                    {capability}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </aside>
  );
}

function ConfigControl({
  field,
  value,
  defaultValue,
  secretOptions,
  sourceFields,
  nodeConfig,
  onChange,
  onConfigChange,
}: {
  field: ConfigField;
  value: unknown;
  defaultValue: unknown;
  secretOptions: readonly { value: string; label: string }[];
  sourceFields: readonly DataField[];
  nodeConfig: Record<string, unknown>;
  onChange: (value: unknown) => void;
  onConfigChange: (values: Record<string, unknown>) => void;
}) {
  const { t } = useTranslation();
  const displayLabel =
    field.kind === "switch-cases" ? t("switchCases.cases")
    : field.kind === "form-builder" ? t("formBuilder.layout")
    : field.label;
  const resolvedValue = value ?? defaultValue;
  const stringValue =
    (field.kind === "json" || field.kind === "type-spec") && typeof resolvedValue !== "string"
      ? JSON.stringify(resolvedValue ?? {}, null, 2)
      : String(resolvedValue ?? "");
  if (field.kind === "sql-parameters") return null;
  return (
    <div className="block">
      <span className="mb-1.5 flex items-center gap-1 text-xs font-medium text-zinc-400">
        {displayLabel}
        {field.required && <span className="text-zinc-600">*</span>}
        {(field.kind === "string" || field.kind === "textarea" || field.kind === "json" || field.kind === "type-spec") && (
          <TextEditorExpandButton onChange={onChange} value={stringValue} placeholder={field.placeholder} multiline={field.kind !== "string"} />
        )}
      </span>
      {field.kind === "javascript-editor" ? (
        <JavaScriptCodeControl
          config={{ ...nodeConfig, [field.name]: resolvedValue }}
          onChange={(config) => onConfigChange(config as unknown as Record<string, unknown>)}
        />
      ) : field.kind === "sql-editor" ? (
        <SQLCodeControl config={nodeConfig} onChange={onConfigChange} />
      ) : field.kind === "database-select" ? (
        <DatabaseSelectControl value={stringValue} onChange={onChange} ariaLabel={field.label} />
      ) : field.kind === "twitch-identity" ? (
        <TwitchIdentitySelect
          value={stringValue}
          onValueChange={onChange}
          ariaLabel={field.label}
        />
      ) : field.kind === "select" || field.kind === "wire-representation" || field.kind === "chat-mode" ? (
        <Select
          value={stringValue}
          onValueChange={onChange}
          options={asArray(field.options).map((option) => ({
            value: option.value,
            label: option.label,
          }))}
          placeholder={
            field.placeholder || `Select ${field.label.toLowerCase()}`
          }
          ariaLabel={field.label}
        />
      ) : field.name === "hotkey" ? (
        <ShortcutRecorder
          value={stringValue}
          ariaLabel={field.label}
          onValueChange={onChange}
        />
      ) : field.kind === "http-headers" ? (
        <HTTPHeadersEditor value={resolvedValue} onChange={onChange} />
      ) : field.kind === "boolean" || field.kind === "http-user-agent-toggle" ? (
        <div className="flex h-9 items-center justify-between rounded-md border border-zinc-800 bg-zinc-900/40 px-2.5">
          <span className="text-xs text-zinc-500">
            {resolvedValue ? t("editor.enabled") : t("editor.disabled")}
          </span>
          <Switch
            checked={Boolean(resolvedValue)}
            onCheckedChange={onChange}
            label={field.label}
          />
        </div>
      ) : field.kind === "route-options" ? (
        <RouteOptionsEditor value={resolvedValue} onChange={onChange} />
      ) : field.kind === "switch-cases" ? (
        <BlueprintSwitchCasesEditor
          value={
            value === undefined && nodeConfig.options !== undefined
              ? undefined
              : resolvedValue
          }
          legacyOptions={nodeConfig.options}
          onChange={onChange}
        />
      ) : field.kind === "form-builder" ? (
        <FormBuilderEditor value={resolvedValue} onChange={onChange} />
      ) : field.kind === "field-outputs" ? (
        <FieldOutputsEditor
          value={resolvedValue}
          sourceFields={sourceFields}
          onChange={onChange}
        />
      ) : field.kind === "html-extractions" ? (
        <HtmlExtractionsEditor value={resolvedValue} onChange={onChange} />
      ) : field.kind === "object-fields" ? (
        <ObjectFieldsEditor value={resolvedValue} onChange={onChange} />
      ) : field.kind === "json-schema" ? (
        <SchemaEditor value={resolvedValue} onChange={onChange} />
      ) : field.kind === "secret" || field.secret ? (
        <Select
          value={stringValue}
          onValueChange={onChange}
          options={secretOptions}
          placeholder={t("editor.secretPlaceholder")}
          ariaLabel={field.label}
        />
      ) : field.kind === "tags" ? (
        <TagsEditor value={stringValue} onChange={onChange} />
      ) : field.kind === "textarea" || field.kind === "json" || field.kind === "type-spec" ? (
        <TextEditorField
          value={stringValue}
          onChange={onChange}
          placeholder={field.placeholder}
          multiline
          ariaLabel={field.label}
        />
      ) : field.kind === "number" ? (
        <Input
          type="number"
          value={stringValue}
          onChange={(event) => onChange(event.target.value)}
          placeholder={field.placeholder}
        />
      ) : (
        <TextEditorField
          value={stringValue}
          onChange={onChange}
          placeholder={field.placeholder}
          ariaLabel={field.label}
        />
      )}
    </div>
  );
}

interface HTTPHeaderValue {
  id: string;
  name: string;
  value: string;
}

function httpHeadersFromValue(value: unknown): HTTPHeaderValue[] {
  if (Array.isArray(value)) {
    return value.flatMap((entry, index) => {
      if (!entry || typeof entry !== "object" || Array.isArray(entry)) return [];
      const record = entry as Record<string, unknown>;
      const name = typeof record.name === "string" ? record.name : typeof record.key === "string" ? record.key : "";
      const headerValue = typeof record.value === "string" ? record.value : "";
      const id = typeof record.id === "string" && record.id ? record.id : `header-${index + 1}`;
      return [{ id, name, value: headerValue }];
    });
  }
  if (!value || typeof value !== "object") return [];
  return Object.entries(value as Record<string, unknown>).flatMap(([name, headerValue], index) =>
    typeof headerValue === "string"
      ? [{ id: `header-${index + 1}`, name, value: headerValue }]
      : [],
  );
}

function nextHTTPHeaderID(headers: readonly HTTPHeaderValue[]) {
  const ids = new Set(headers.map((header) => header.id));
  for (let index = 1; ; index += 1) {
    const id = `header-${index}`;
    if (!ids.has(id)) return id;
  }
}

function HTTPHeadersEditor({
  value,
  onChange,
}: {
  value: unknown;
  onChange: (value: HTTPHeaderValue[]) => void;
}) {
  const { t } = useTranslation();
  const headers = httpHeadersFromValue(value);
  const update = (index: number, change: Partial<HTTPHeaderValue>) => {
    onChange(headers.map((header, current) => current === index ? { ...header, ...change } : header));
  };

  return (
    <div className="rounded-md border border-zinc-800 bg-zinc-900/30 p-2.5">
      <p className="mb-2 text-[11px] leading-4 text-zinc-500">
        {t("editor.httpHeadersHelp")}
      </p>
      <div className="space-y-2">
        {headers.map((header, index) => (
          <div key={header.id} className="grid grid-cols-[minmax(0,.8fr)_minmax(0,1.2fr)_auto] items-center gap-1.5">
            <Input
              value={header.name}
              onChange={(event) => update(index, { name: event.target.value })}
              placeholder={t("editor.headerName")}
              aria-label={`${t("editor.headerName")} ${index + 1}`}
              className="font-mono text-xs"
            />
            <Input
              value={header.value}
              onChange={(event) => update(index, { value: event.target.value })}
              placeholder={t("editor.headerValue")}
              aria-label={`${t("editor.headerValue")} ${index + 1}`}
              className="font-mono text-xs"
            />
            <button
              type="button"
              onClick={() => onChange(headers.filter((_, current) => current !== index))}
              className="rounded p-1.5 text-zinc-500 transition-colors hover:bg-zinc-800 hover:text-zinc-100 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-zinc-400"
              aria-label={`${t("editorActions.delete")} ${t("editor.headerName")} ${index + 1}`}
            >
              <Trash2 className="size-3.5" />
            </button>
          </div>
        ))}
      </div>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="mt-2 h-8"
        onClick={() => onChange([...headers, { id: nextHTTPHeaderID(headers), name: "", value: "" }])}
      >
        <Plus className="size-3.5" />
        {t("editor.addHeader")}
      </Button>
    </div>
  );
}

function tagsFromValue(value: string): string[] {
  const seen = new Set<string>();
  return value
    .split(/[,;\n\r]/)
    .map((tag) => tag.trim().replace(/^#/, "").replace(/\s+/g, " "))
    .filter((tag) => {
      const key = tag.toLowerCase();
      if (!tag || seen.has(key)) return false;
      seen.add(key);
      return true;
    });
}

function TagsEditor({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState("");
  const tags = tagsFromValue(value);
  const commit = () => {
    const next = tagsFromValue([...tags, draft].join(", "));
    if (next.length !== tags.length) onChange(next.join(", "));
    setDraft("");
  };
  const remove = (tag: string) =>
    onChange(tags.filter((item) => item !== tag).join(", "));

  return (
    <div className="rounded-md border border-zinc-700 bg-zinc-950 p-2">
      {tags.length > 0 ? (
        <div className="mb-2 flex flex-wrap gap-1.5">
          {tags.map((tag) => (
            <span
              key={tag.toLowerCase()}
              className="inline-flex items-center gap-1 rounded bg-zinc-800 py-1 pl-2 pr-1 text-[11px] text-zinc-300"
            >
              {tag}
              <button
                type="button"
                aria-label={t("editorActions.delete")}
                onClick={() => remove(tag)}
                className="rounded p-0.5 text-zinc-500 hover:bg-zinc-700 hover:text-zinc-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-zinc-400"
              >
                <X className="size-3" />
              </button>
            </span>
          ))}
        </div>
      ) : null}
      <input
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={commit}
        onKeyDown={(event) => {
          if (event.key !== "Enter" && event.key !== ",") return;
          event.preventDefault();
          commit();
        }}
        placeholder={t("editor.addTag")}
        className="h-7 w-full bg-transparent px-1 text-xs text-zinc-200 outline-none placeholder:text-zinc-600"
      />
    </div>
  );
}

type SchemaType = "object" | "array" | "string" | "number" | "boolean";

interface SchemaProperty {
  name: string;
  type: SchemaType;
  required: boolean;
  description: string;
}

interface ObjectSchemaValue {
  type: SchemaType;
  properties: SchemaProperty[];
}

const schemaTypes: { value: SchemaType; label: string }[] = [
  { value: "object", label: "Object" },
  { value: "array", label: "Array" },
  { value: "string", label: "Text" },
  { value: "number", label: "Number" },
  { value: "boolean", label: "True / false" },
];

function parseStructuredValue(value: unknown): unknown {
  if (typeof value !== "string") return value;
  try {
    return JSON.parse(value);
  } catch {
    return undefined;
  }
}

function nextRouteOptionID(options: RouteOptionValue[]) {
  let index = options.length + 1;
  let id = `option-${index}`;
  const used = new Set(options.map((option) => option.id));
  while (used.has(id)) {
    index++;
    id = `option-${index}`;
  }
  return id;
}

function RouteOptionsEditor({
  value,
  onChange,
}: {
  value: unknown;
  onChange: (value: RouteOptionValue[]) => void;
}) {
  const { t } = useTranslation();
  const options = routeOptionsFromValue(value);
  const duplicateIDs = options.filter(
    (option, index) =>
      options.findIndex((candidate) => candidate.id === option.id) !== index,
  );
  const update = (index: number, patch: Partial<RouteOptionValue>) => {
    onChange(
      options.map((option, current) =>
        current === index ? { ...option, ...patch } : option,
      ),
    );
  };

  return (
    <div className="space-y-3 rounded-md border border-zinc-800 bg-zinc-900/30 p-3">
      <p className="text-[11px] leading-4 text-zinc-500">
        {t("editorExtra.optionHelp")}
      </p>
      {options.map((option, index) => (
        <article
          key={`${option.id}-${index}`}
          className="rounded-md border border-zinc-800 bg-zinc-950/60 p-2.5"
        >
          <div className="mb-2 flex items-center justify-between">
            <div>
              <span className="text-[10px] font-semibold uppercase tracking-[.12em] text-zinc-600">
                {t("editorExtra.option")} {index + 1}
              </span>
              <p className="mt-0.5 text-[10px] text-zinc-600">
                {t("editorExtra.optionCreatesOutput")}
              </p>
            </div>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="size-6 p-0 text-zinc-500 hover:text-red-300"
              aria-label={t("editorActions.delete")}
              onClick={() =>
                onChange(options.filter((_, current) => current !== index))
              }
            >
              <Trash2 className="size-3.5" />
            </Button>
          </div>
          <label className="block text-[11px] font-medium text-zinc-400">
            {t("editor.outputId")}
            <Input
              value={option.id}
              onChange={(event) => update(index, { id: event.target.value })}
              placeholder={t("editorExtra.exampleOptionID")}
              aria-label={t("editor.outputId")}
              className="mt-1 font-mono text-xs"
            />
          </label>
          <label className="mt-2 block text-[11px] font-medium text-zinc-400">
            {t("editor.displayName")}
            <Input
              value={option.label}
              onChange={(event) => update(index, { label: event.target.value })}
              placeholder={t("editorExtra.exampleOptionName")}
              aria-label={t("editor.displayName")}
              className="mt-1"
            />
          </label>
          <label className="mt-2 block text-[11px] font-medium text-zinc-400">
            {t("editor.guidance")} <span className="font-normal text-zinc-600">({t("editor.optional")})</span>
            <textarea
              value={option.description}
              onChange={(event) => update(index, { description: event.target.value })}
              placeholder={t("editor.guidancePlaceholder")}
              aria-label={t("editor.guidance")}
              className="mt-1 min-h-16 w-full resize-y rounded-md border border-zinc-700 bg-zinc-950 px-2.5 py-2 text-xs leading-5 text-zinc-200 outline-none placeholder:text-zinc-600 focus:border-zinc-500"
            />
          </label>
        </article>
      ))}
      {duplicateIDs.length > 0 ? (
        <p className="text-[11px] text-red-300">{t("editor.optionIdsUnique")}</p>
      ) : null}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        onClick={() => {
          const id = nextRouteOptionID(options);
          onChange([
            ...options,
            { id, label: `${t("editorExtra.option")} ${options.length + 1}`, description: "" },
          ]);
        }}
      >
        <Plus className="size-3.5" />
        {t("editor.addOption")}
      </Button>
    </div>
  );
}

function schemaFromValue(value: unknown): ObjectSchemaValue {
  const parsed = parseStructuredValue(value);
  if (!isRecord(parsed)) return { type: "object", properties: [] };
  const type = schemaTypes.some((item) => item.value === parsed.type)
    ? (parsed.type as SchemaType)
    : "object";
  const required = new Set(
    Array.isArray(parsed.required)
      ? parsed.required.filter(
          (item): item is string => typeof item === "string",
        )
      : [],
  );
  const properties = isRecord(parsed.properties)
    ? Object.entries(parsed.properties).map(([name, property]) => ({
        name,
        type:
          isRecord(property) &&
          schemaTypes.some((item) => item.value === property.type)
            ? (property.type as SchemaType)
            : "string",
        required: required.has(name),
        description:
          isRecord(property) && typeof property.description === "string"
            ? property.description
            : "",
      }))
    : [];
  return { type, properties };
}

function schemaToValue(schema: ObjectSchemaValue): Record<string, unknown> {
  const properties = Object.fromEntries(
    schema.properties
      .filter((property) => property.name.trim())
      .map((property) => {
        const description = property.description.trim();
        return [
          property.name.trim(),
          {
            type: property.type,
            ...(description ? { description } : {}),
          },
        ];
      }),
  );
  const required = schema.properties
    .filter((property) => property.required && property.name.trim())
    .map((property) => property.name.trim());
  if (schema.type !== "object") return { type: schema.type };
  return {
    type: "object",
    properties,
    ...(required.length > 0 ? { required } : {}),
  };
}

function SchemaPropertyCard({
  property,
  index,
  onUpdate,
  onRemove,
}: {
  property: SchemaProperty;
  index: number;
  onUpdate: (patch: Partial<SchemaProperty>) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const schemaTypeOptions = useMemo(
    () => schemaTypes.map((item) => ({ ...item, label: item.value === "array" ? t("editor.list") : item.value === "string" ? t("editor.text") : t(`editor.${item.value}`) })),
    [t],
  );
  return (
    <article className="rounded-md border border-zinc-800 bg-zinc-950/60 p-2.5">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-[10px] font-semibold uppercase tracking-[.12em] text-zinc-600">
          {t("editorExtra.field")} {index + 1}
        </span>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="size-6 p-0 text-zinc-500 hover:text-red-300"
          aria-label={t("editorActions.delete")}
          onClick={onRemove}
        >
          <Trash2 className="size-3.5" />
        </Button>
      </div>
      <label className="block text-[11px] font-medium text-zinc-400">
        {t("editorExtra.fieldName")}
        <Input
          value={property.name}
          onChange={(event) => onUpdate({ name: event.target.value })}
          placeholder={t("editorExtra.exampleFieldName")}
          aria-label={t("editorExtra.fieldName")}
          className="mt-1 font-mono text-xs"
        />
      </label>
      <label className="mt-2 block text-[11px] font-medium text-zinc-400">
        {t("editor.guidance")} <span className="font-normal text-zinc-600">({t("editor.optional")})</span>
        <textarea
          value={property.description}
          onChange={(event) => onUpdate({ description: event.target.value })}
          placeholder={t("editor.fieldGuidancePlaceholder")}
          aria-label={t("editor.guidance")}
          className="mt-1 min-h-16 w-full resize-y rounded-md border border-zinc-700 bg-zinc-950 px-2.5 py-2 text-xs leading-5 text-zinc-200 outline-none placeholder:text-zinc-600 focus:border-zinc-500"
        />
      </label>
      <div className="mt-2 flex items-end justify-between gap-3">
        <label className="min-w-0 flex-1 text-[11px] font-medium text-zinc-400">
          {t("editor.valueType")}
          <Select
            value={property.type}
            onValueChange={(type) => onUpdate({ type: type as SchemaType })}
            options={schemaTypeOptions.filter((item) => item.value !== "object")}
            ariaLabel={t("editor.valueType")}
            className="mt-1"
          />
        </label>
        <div className="flex h-8 shrink-0 items-center gap-2 rounded-md border border-zinc-800 bg-zinc-900/50 px-2">
          <span className="text-[11px] text-zinc-400">{t("editor.required")}</span>
          <Switch
            checked={property.required}
            onCheckedChange={(required) => onUpdate({ required })}
            label={t("editor.required")}
          />
        </div>
      </div>
    </article>
  );
}

function SchemaEditor({
  value,
  onChange,
}: {
  value: unknown;
  onChange: (value: Record<string, unknown>) => void;
}) {
  const { t } = useTranslation();
  const schemaTypeOptions = useMemo(
    () => schemaTypes.map((item) => ({ ...item, label: item.value === "array" ? t("editor.list") : item.value === "string" ? t("editor.text") : t(`editor.${item.value}`) })),
    [t],
  );
  const schema = schemaFromValue(value);
  const update = (patch: Partial<ObjectSchemaValue>) => {
    const next = { ...schema, ...patch };
    onChange(schemaToValue(next));
  };
  const updateProperty = (index: number, patch: Partial<SchemaProperty>) => {
    update({
      properties: schema.properties.map((property, current) =>
        current === index ? { ...property, ...patch } : property,
      ),
    });
  };

  return (
    <div className="space-y-3 rounded-md border border-zinc-800 bg-zinc-900/30 p-3">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs font-medium text-zinc-400">{t("editor.responseShape")}</span>
        <Select
          value={schema.type}
          onValueChange={(type) => update({ type: type as SchemaType })}
          options={schemaTypeOptions}
          ariaLabel={t("editor.responseShape")}
          className="w-32 shrink-0"
        />
      </div>
      {schema.type === "object" ? (
        <>
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-zinc-300">{t("editor.fields")}</span>
            <span className="text-[10px] text-zinc-600">{schema.properties.length}</span>
          </div>
          {schema.properties.map((property, index) => (
            <SchemaPropertyCard
              key={`${property.name}-${index}`}
              property={property}
              index={index}
              onUpdate={(patch) => updateProperty(index, patch)}
              onRemove={() => update({ properties: schema.properties.filter((_, current) => current !== index) })}
            />
          ))}
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="w-full border border-dashed border-zinc-700 text-zinc-300"
            onClick={() => {
              const existing = new Set(
                schema.properties.map((property) => property.name),
              );
              let index = schema.properties.length + 1;
              let name = `field${index}`;
              while (existing.has(name)) {
                index++;
                name = `field${index}`;
              }
              update({
                properties: [
                  ...schema.properties,
                  { name, type: "string", required: true, description: "" },
                ],
              });
            }}
          >
            <Plus className="size-3.5" />
            {t("editor.addField")}
          </Button>
        </>
      ) : (
        <p className="rounded-md border border-zinc-800 bg-zinc-950/60 px-2.5 py-2 text-[11px] leading-4 text-zinc-500">The model returns one {schema.type} value.</p>
      )}
    </div>
  );
}

function hydrateNodes(
  nodes: FlowNode[],
  definitions: NodeDefinition[],
): EditorNode[] {
  return asArray(nodes).map((node) => {
    const data = isRecord(node.data) ? node.data : {};
    const config = isRecord(data.config) ? data.config : data;
    const type = String(data.type ?? node.type);
    const definition = definitions.find((item) => item.type === type);
    return {
      ...node,
      type: "neuropipe",
      data: {
        ...data,
        type,
        label: definition?.label,
        icon: definition?.icon,
                inputs: resolveInputs(definition, config),
                outputs: resolveOutputs(definition, config),
                resolvedDefinition: definition,
                config,
      },
    } as EditorNode;
  });
}

function createEditorNode(
  definition: NodeDefinition,
  position: { x: number; y: number },
): EditorNode {
  const config = structuredClone(
    isRecord(definition.defaultConfig) ? definition.defaultConfig : {},
  );
  return {
    id: `${definition.type.replace(":", "-")}-${crypto.randomUUID().slice(0, 8)}`,
    type: "neuropipe",
    position,
    data: {
      type: definition.type,
      label: definition.label,
      icon: definition.icon,
      inputs: resolveInputs(definition, config),
      outputs: resolveOutputs(definition, config),
                resolvedDefinition: definition,
      config,
    },
  };
}

function resolveOutputs(
  definition: NodeDefinition | undefined,
  config: Record<string, unknown>,
): NodePort[] {
  return resolveConfigDrivenOutputs(definition, config);
}

function resolveInputs(
  definition: NodeDefinition | undefined,
  config: Record<string, unknown>,
): NodePort[] {
  return resolveConfigDrivenInputs(definition, config);
}

function dehydrateNodes(nodes: EditorNode[]): FlowNode[] {
  return nodes.map((node) => ({
    ...node,
    type: node.data.type,
    data: { config: node.data.config },
  }));
}

function parseFieldValue(field: ConfigField, value: unknown): unknown {
  if (field.kind === "number") return Number(value);
  if ((field.kind === "json" || field.kind === "type-spec") && typeof value === "string") {
    try {
      return JSON.parse(value);
    } catch {
      return value;
    }
  }
  return value;
}

// Inspector edits are the explicit conversion boundary for the Constant node:
// persisted V3 configuration stores canonical JSON values, never number or
// Boolean text that the runtime has to silently parse.
function normalizeNodeConfig(type: string, config: Record<string, unknown>) {
  if (type !== "data:constant") return config;
  const target = config.type;
  const value = config.value;
  if (target === "number" && typeof value === "string") {
    const number = Number(value);
    return Number.isFinite(number) ? { ...config, value: number } : config;
  }
  if (target === "boolean" && typeof value === "string") {
    if (value.trim().toLowerCase() === "true") return { ...config, value: true };
    if (value.trim().toLowerCase() === "false") return { ...config, value: false };
  }
  return config;
}
