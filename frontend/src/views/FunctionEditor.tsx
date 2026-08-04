import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
  reconnectEdge,
  type Connection,
  type EdgeChange,
  type NodeChange,
  type NodeProps,
  type ReactFlowInstance,
} from "@xyflow/react";
import {
  ArrowLeft,
  Braces,
  Copy,
  LayoutGrid,
  Loader2,
  Magnet,
  PanelLeft,
  Plus,
  Save,
  Trash2,
  UploadCloud,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { BlueprintContextMenu } from "@/components/BlueprintContextMenu";
import {
  FieldOutputsEditor,
  ObjectFieldsEditor,
} from "@/components/BlueprintDataMappingsEditor";
import { BlueprintSwitchCasesEditor } from "@/components/BlueprintSwitchCasesEditor";
import { BlueprintNodeLibrary } from "@/components/BlueprintNodeLibrary";
import { BlueprintPinTooltip } from "@/components/BlueprintPinTooltip";
import { IconAppearancePicker, LucideIcon, LucideIconPicker } from "@/components/LucideIconPicker";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Tooltip } from "@/components/ui/tooltip";
import { desktop } from "@/lib/bridge";
import { nodePinColor } from "@/lib/node-pins";
import {
  resolveConfigDrivenInputs,
  resolveConfigDrivenOutputs,
} from "@/lib/blueprint-dynamic-pins";
import { usePersistedChoice } from "@/lib/preferences";
import { cn } from "@/lib/utils";
import type {
  CustomFunction,
  DataField,
  DataType,
  FlowEdge,
  FlowNode,
  FunctionPin,
  NodeDefinition,
  NodePort,
} from "@/lib/types";
import { useConfirmationStore } from "@/stores/confirmation";
import { useUIStore } from "@/stores/ui";
import { useTranslation } from "react-i18next";

type EditorNode = FlowNode & {
  data: {
    type: string;
    label: string;
    inputs: NodePort[];
    outputs: NodePort[];
    config: Record<string, unknown>;
  };
};
interface FunctionCanvasMenu {
  x: number;
  y: number;
  position: { x: number; y: number };
  nodeID?: string;
  edgeID?: string;
  source?: string;
  sourceHandle?: string | null;
}

interface ReconnectState {
  edgeID: string;
  allowed: boolean;
  completed: boolean;
}

const functionLibraryCollapsedCategoriesKey =
  "neuropipe.function-editor.library.collapsed-categories.v1";
const functionGridSnapModes = ["on", "off"] as const;
const editorSnapGrid: [number, number] = [20, 20];
const defaultEdgeOptions = { reconnectable: "target" as const };
const dataTypes: DataType[] = [
  "any",
  "text",
  "number",
  "boolean",
  "object",
  "list",
];

function compatiblePins(source: NodePort, target: NodePort) {
  return (
    source.kind === target.kind &&
    (source.kind === "exec" ||
      source.dataType === "any" ||
      target.dataType === "any" ||
      source.dataType === target.dataType)
  );
}

function FunctionGraphNode({ data, selected }: NodeProps<EditorNode>) {
  return (
    <div
      className={cn(
        "min-w-48 rounded-lg border bg-zinc-950 shadow-xl",
        selected ? "border-zinc-100 ring-2 ring-white/10" : "border-zinc-700",
      )}
    >
      <div className="border-b border-zinc-800 px-3 py-2 text-xs font-medium text-zinc-100">
        {data.label}
      </div>
      <div className="py-1">
        {data.inputs.map((pin) => (
          <div
            key={pin.id}
            className="relative flex min-h-6 items-center px-3 text-[10px] text-zinc-400"
          >
            <Handle
              id={pin.id}
              type="target"
              position={Position.Left}
              className={
                pin.kind === "exec" ? "!h-3 !w-3 !rounded-sm" : "!size-2.5"
              }
              style={{ background: nodePinColor(pin), left: 0 }}
            />
            <BlueprintPinTooltip pin={pin} target />
          </div>
        ))}
      </div>
      <div className="border-t border-zinc-800 py-1">
        {data.outputs.map((pin) => (
          <div
            key={pin.id}
            className="relative flex min-h-6 items-center justify-end px-3 text-[10px] text-zinc-400"
          >
            <BlueprintPinTooltip pin={pin} />
            <Handle
              id={pin.id}
              type="source"
              position={Position.Right}
              className={
                pin.kind === "exec" ? "!h-3 !w-3 !rounded-sm" : "!size-2.5"
              }
              style={{ background: nodePinColor(pin), right: 0 }}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
const graphTypes = { functionNode: FunctionGraphNode };

export function FunctionEditor({
  functionID,
  definitions,
  onRefresh,
}: {
  functionID: string;
  definitions: NodeDefinition[];
  onRefresh: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const { setError, setScreen } = useUIStore();
  const requestConfirmation = useConfirmationStore((state) => state.ask);
  const [item, setItem] = useState<CustomFunction>();
  const [nodes, setNodes] = useState<EditorNode[]>([]);
  const [edges, setEdges] = useState<FlowEdge[]>([]);
  const [selectedID, setSelectedID] = useState<string>();
  const [libraryQuery, setLibraryQuery] = useState("");
  const [showLibrary, setShowLibrary] = useState(true);
  const [menu, setMenu] = useState<FunctionCanvasMenu>();
  const [menuQuery, setMenuQuery] = useState("");
  const [connecting, setConnecting] = useState<{ source: string; sourceHandle: string | null }>();
  const [busy, setBusy] = useState("");
  const [dirty, setDirty] = useState(false);
  const [gridSnapMode, setGridSnapMode] = usePersistedChoice(
    "neuropipe.function-editor.grid-snap.v1",
    functionGridSnapModes,
    "on",
  );
  const [flow, setFlow] = useState<ReactFlowInstance<EditorNode, FlowEdge>>();
  const canvasRef = useRef<HTMLDivElement>(null);
  const reconnectRef = useRef<ReconnectState>();
  const load = useCallback(async () => {
    try {
      setBusy("load");
      const next = await desktop.getFunction(functionID);
      setItem(next);
      setNodes(hydrate(next, definitions));
      setEdges(next.draftDefinition.edges ?? []);
      setDirty(false);
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : t("functionEditor.loadFailed"),
      );
    } finally {
      setBusy("");
    }
  }, [definitions, functionID, setError, t]);
  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    if (!item) return;
    setNodes((current) =>
      current.map((node) => {
        const definition = definitions.find(
          (entry) => entry.type === node.data.type,
        );
        if (!definition || !isFunctionBoundary(node.data.type)) return node;
        return {
          ...node,
          data: {
            ...node.data,
            inputs: boundaryPins(
              node.data.type,
              definition.inputs,
              item,
              "input",
            ),
            outputs: boundaryPins(
              node.data.type,
              definition.outputs,
              item,
              "output",
            ),
          },
        };
      }),
    );
  }, [definitions, item?.inputs, item?.mode, item?.outputs]);
  const update = (change: Partial<CustomFunction>) => {
    setItem((current) => (current ? { ...current, ...change } : current));
    setDirty(true);
  };
  const availableDefinitions = useMemo(
    () =>
      definitions.filter((definition) => {
        if (definition.type.startsWith("trigger:") || isFunctionBoundary(definition.type)) return false;
        return item?.mode !== "pure" || definition.mode === "pure" || definition.mode === "visual";
      }),
    [definitions, item?.mode],
  );
  const filteredDefinitions = useMemo(
    () =>
      availableDefinitions.filter((definition) =>
        `${definition.label} ${definition.category} ${definition.description}`
          .toLowerCase()
          .includes(libraryQuery.toLowerCase()),
      ),
    [availableDefinitions, libraryQuery],
  );
  const selected = nodes.find((node) => node.id === selectedID);
  const selectedDefinition = definitions.find(
    (definition) => definition.type === selected?.data.type,
  );
  const selectedSourceFields = useMemo((): DataField[] => {
    if (
      selected?.data.type !== "data:get_field" &&
      selected?.data.type !== "data:break_object"
    )
      return [];
    const edge = edges.find(
      (item) =>
        item.target === selected.id &&
        item.targetHandle === "source" &&
        item.kind === "data",
    );
    if (!edge) return [];
    return (
      nodes
        .find((node) => node.id === edge.source)
        ?.data.outputs.find((pin) => pin.id === edge.sourceHandle)?.fields ?? []
    );
  }, [edges, nodes, selected]);
  const updateNodeConfig = (field: string, value: unknown) => {
    if (!selectedID) return;
    const selectedNode = nodes.find((node) => node.id === selectedID);
    const nextConfig = selectedNode
      ? { ...selectedNode.data.config, [field]: value }
      : undefined;
    const removedOutputIDs = nextConfig
      ? selectedNode!.data.outputs
          .filter((output) => {
            const definition = definitions.find(
              (item) => item.type === selectedNode!.data.type,
            );
            return !graphOutputs(
              selectedNode!.data.type,
              definition,
              item,
              nextConfig,
            ).some((next) => next.id === output.id);
          })
          .map((output) => output.id)
      : [];
    setNodes((current) =>
      current.map((node) => {
        if (node.id !== selectedID) return node;
        const config = { ...node.data.config, [field]: value };
        const definition = definitions.find((item) => item.type === node.data.type);
        return {
          ...node,
          data: {
            ...node.data,
            inputs: graphInputs(node.data.type, definition, item, config),
            outputs: graphOutputs(node.data.type, definition, item, config),
            config,
          },
        };
      }),
    );
    if (removedOutputIDs.length > 0) {
      setEdges((current) =>
        current.filter(
          (edge) =>
            edge.source !== selectedID ||
            !removedOutputIDs.includes(edge.sourceHandle ?? "out"),
        ),
      );
    }
    setDirty(true);
  };
  const addPin = (side: "inputs" | "outputs") =>
    update({
      [side]: [
        ...(item?.[side] ?? []),
        {
          id: crypto.randomUUID(),
          name: side === "inputs" ? "Input" : "Output",
          dataType: "any",
        },
      ],
    } as Pick<CustomFunction, typeof side>);
  const updatePin = (
    side: "inputs" | "outputs",
    index: number,
    change: Partial<FunctionPin>,
  ) =>
    update({
      [side]: (item?.[side] ?? []).map((pin, pinIndex) =>
        pinIndex === index ? { ...pin, ...change } : pin,
      ),
    } as Pick<CustomFunction, typeof side>);
  const removePin = (side: "inputs" | "outputs", id: string) =>
    update({
      [side]: (item?.[side] ?? []).filter((pin) => pin.id !== id),
    } as Pick<CustomFunction, typeof side>);
  const onNodesChange = (changes: NodeChange<EditorNode>[]) => {
    setNodes((current) => applyNodeChanges(changes, current));
    if (changes.some((change) => change.type !== "select")) setDirty(true);
  };
  const onEdgesChange = (changes: EdgeChange<FlowEdge>[]) => {
    setEdges((current) => applyEdgeChanges(changes, current));
    if (changes.some((change) => change.type !== "select")) setDirty(true);
  };
  const isValidConnection = useCallback(
    (connection: Connection | FlowEdge) => {
      if (!connection.source || !connection.target || !connection.sourceHandle || !connection.targetHandle) return false;
      const source = nodes.find((node) => node.id === connection.source);
      const target = nodes.find((node) => node.id === connection.target);
      const sourcePin = source?.data.outputs.find((pin) => pin.id === connection.sourceHandle);
      const targetPin = target?.data.inputs.find((pin) => pin.id === connection.targetHandle);
      if (!sourcePin || !targetPin || !compatiblePins(sourcePin, targetPin)) return false;
      const replacingID = "id" in connection ? connection.id : undefined;
      const incoming = edges.filter(
        (edge) => edge.id !== replacingID && edge.target === connection.target && edge.targetHandle === connection.targetHandle,
      ).length;
      return !targetPin.maxConnections || incoming < targetPin.maxConnections;
    },
    [edges, nodes],
  );
  const onConnect = (connection: Connection) => {
    if (!isValidConnection(connection)) return;
    const source = nodes.find((node) => node.id === connection.source);
    const pin = source?.data.outputs.find(
      (item) => item.id === connection.sourceHandle,
    );
    if (!pin) return;
    setEdges((current) =>
      addEdge(
        {
          ...connection,
          id: crypto.randomUUID(),
          kind: pin.kind,
          animated: pin.kind === "exec",
          markerEnd: pin.kind === "exec" ? { type: MarkerType.ArrowClosed, color: "#fafafa" } : undefined,
          style: {
            stroke: nodePinColor(pin),
            strokeWidth: pin.kind === "exec" ? 2 : 1.5,
          },
        },
        current,
      ),
    );
    setDirty(true);
  };
  const onReconnectStart = useCallback((event: React.MouseEvent, edge: FlowEdge, stableHandle: "source" | "target") => {
    reconnectRef.current = {
      edgeID: edge.id,
      allowed: stableHandle === "source" && (event.ctrlKey || event.metaKey),
      completed: false,
    };
    setConnecting(undefined);
  }, []);
  const onReconnect = useCallback((edge: FlowEdge, connection: Connection) => {
    const reconnecting = reconnectRef.current;
    if (!reconnecting?.allowed || reconnecting.edgeID !== edge.id || !isValidConnection(connection)) return;
    reconnecting.completed = true;
    setEdges((current) => reconnectEdge(edge, connection, current, { shouldReplaceId: false }));
    setDirty(true);
  }, [isValidConnection]);
  const onReconnectEnd = useCallback((_event: MouseEvent | TouchEvent, edge: FlowEdge, _stableHandle: "source" | "target", connectionState: { isValid: boolean | null }) => {
    const reconnecting = reconnectRef.current;
    if (!reconnecting || reconnecting.edgeID !== edge.id) return;
    if (reconnecting.allowed && !reconnecting.completed && connectionState.isValid !== true) {
      setEdges((current) => current.filter((item) => item.id !== edge.id));
      setDirty(true);
    }
    reconnectRef.current = undefined;
  }, []);
  const snapPosition = useCallback((position: { x: number; y: number }) =>
    gridSnapMode === "on"
      ? {
          x: Math.round(position.x / editorSnapGrid[0]) * editorSnapGrid[0],
          y: Math.round(position.y / editorSnapGrid[1]) * editorSnapGrid[1],
        }
      : position,
  [gridSnapMode]);
  const addNode = useCallback((definition: NodeDefinition, position?: { x: number; y: number }) => {
    const id = `${definition.type.replace(":", "-")}-${crypto.randomUUID().slice(0, 8)}`;
    const config = structuredClone(definition.defaultConfig ?? {});
    setNodes((current) => [
      ...current,
      {
        id,
        type: "functionNode",
        position: snapPosition(position ?? { x: 260 + current.length * 35, y: 150 + current.length * 28 }),
        data: {
          type: definition.type,
          label: definition.label,
          inputs: graphInputs(definition.type, definition, item, config),
          outputs: graphOutputs(definition.type, definition, item, config),
          config,
        },
      },
    ]);
    setSelectedID(id);
    setDirty(true);
    return id;
  }, [item, snapPosition]);
  const removeSelected = useCallback(() => {
    if (!selectedID) return;
    setNodes((current) => current.filter((node) => node.id !== selectedID));
    setEdges((current) => current.filter((edge) => edge.source !== selectedID && edge.target !== selectedID));
    setSelectedID(undefined);
    setDirty(true);
  }, [selectedID]);
  const duplicateSelected = useCallback(() => {
    const selected = nodes.find((node) => node.id === selectedID);
    if (!selected) return;
    const duplicate = {
      ...selected,
      id: crypto.randomUUID(),
      position: { x: selected.position.x + 36, y: selected.position.y + 36 },
      data: { ...selected.data, config: structuredClone(selected.data.config) },
      selected: true,
    };
    setNodes((current) => [...current.map((node) => ({ ...node, selected: false })), duplicate]);
    setSelectedID(duplicate.id);
    setDirty(true);
  }, [nodes, selectedID]);
  const autoLayout = useCallback(() => {
    setNodes((current) => current.map((node, index) => ({
      ...node,
      position: { x: 100 + (index % 4) * 260, y: 120 + Math.floor(index / 4) * 180 },
    })));
    setDirty(true);
  }, []);
  const removeEdge = useCallback((edgeID: string) => {
    setEdges((current) => current.filter((edge) => edge.id !== edgeID));
    setDirty(true);
  }, []);
  const insertReroute = useCallback((edgeID: string, position: { x: number; y: number }) => {
    const edge = edges.find((item) => item.id === edgeID);
    if (!edge) return;
    const type = edge.kind === "exec" ? "flow:reroute" : "data:reroute";
    const definition = availableDefinitions.find((item) => item.type === type);
    if (!definition) return;
    const id = addNode(definition, { x: position.x - 10, y: position.y - 10 });
    const inputHandle = edge.kind === "exec" ? "in" : "value";
    const outputHandle = edge.kind === "exec" ? "out" : "value";
    setEdges((current) => current.flatMap((item) => item.id === edgeID ? [
      { ...item, id: crypto.randomUUID(), target: id, targetHandle: inputHandle, selected: false },
      { ...item, id: crypto.randomUUID(), source: id, sourceHandle: outputHandle, selected: false },
    ] : [item]));
    setDirty(true);
  }, [addNode, availableDefinitions, edges]);
  const openMenu = (
    event: { clientX: number; clientY: number; preventDefault: () => void },
    nodeID?: string,
    edgeID?: string,
    connection?: { source: string; sourceHandle: string | null },
  ) => {
    event.preventDefault();
    const bounds = canvasRef.current?.getBoundingClientRect();
    const width = 332;
    const height = 450;
    const gutter = 8;
    const relativeX = bounds ? event.clientX - bounds.left : event.clientX;
    const relativeY = bounds ? event.clientY - bounds.top : event.clientY;
    setMenu({
      x: bounds ? Math.max(gutter, Math.min(relativeX, Math.max(gutter, bounds.width - width - gutter))) : relativeX,
      y: bounds ? Math.max(gutter, Math.min(relativeY, Math.max(gutter, bounds.height - height - gutter))) : relativeY,
      position: flow?.screenToFlowPosition({ x: event.clientX, y: event.clientY }) ?? { x: relativeX, y: relativeY },
      nodeID,
      edgeID,
      source: connection?.source,
      sourceHandle: connection?.sourceHandle,
    });
    setMenuQuery("");
  };
  const addFromMenu = useCallback((definition: NodeDefinition) => {
    if (!menu) return;
    const id = addNode(definition, menu.position);
    if (menu.source) {
      const source = nodes.find((node) => node.id === menu.source);
      const pin = source?.data.outputs.find((item) => item.id === menu.sourceHandle);
      const target =
        pin &&
        graphInputs(
          definition.type,
          definition,
          item,
          structuredClone(definition.defaultConfig ?? {}),
        ).find((input) => compatiblePins(pin, input));
      if (pin && target) {
        setEdges((current) => addEdge({
          id: crypto.randomUUID(), source: menu.source!, sourceHandle: pin.id, target: id, targetHandle: target.id,
          kind: pin.kind, animated: pin.kind === "exec", markerEnd: pin.kind === "exec" ? { type: MarkerType.ArrowClosed, color: "#fafafa" } : undefined,
          style: { stroke: nodePinColor(pin), strokeWidth: pin.kind === "exec" ? 2 : 1.5 },
        }, current));
      }
    }
    setMenu(undefined);
    setConnecting(undefined);
  }, [addNode, menu, nodes]);
  const drop = (event: React.DragEvent) => {
    event.preventDefault();
    const type = event.dataTransfer.getData("application/neuropipe-function-node");
    const definition = availableDefinitions.find((entry) => entry.type === type);
    if (definition && flow) addNode(definition, flow.screenToFlowPosition({ x: event.clientX, y: event.clientY }, { snapToGrid: gridSnapMode === "on", snapGrid: editorSnapGrid }));
  };
  const save = async (publish = false) => {
    if (!item) return;
    try {
      setBusy(publish ? "publish" : "save");
      const draftDefinition = {
        schemaVersion: 2,
        nodes: nodes.map((node) => ({
          id: node.id,
          type: node.data.type,
          position: node.position,
          data: { config: node.data.config },
        })),
        edges,
        viewport: item.draftDefinition.viewport ?? { x: 0, y: 0, zoom: 1 },
      };
      const saved = await desktop.saveFunction({ ...item, draftDefinition });
      const result = publish ? await desktop.publishFunction(saved) : saved;
      setItem(result);
      setDirty(false);
      await onRefresh();
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : t("functionEditor.saveFailed"),
      );
    } finally {
      setBusy("");
    }
  };
  const destroy = async () => {
    if (
      !item ||
      !(await requestConfirmation({
        title: t("functionEditor.deleteTitle"),
        description: t("functionEditor.deleteDescription", { name: item.name }),
        confirmLabel: t("functionEditor.deleteConfirm"),
      }))
    )
      return;
    try {
      setBusy("delete");
      await desktop.deleteFunction(item.id);
      await onRefresh();
      setScreen("functions");
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : t("functionEditor.deleteFailed"),
      );
    } finally {
      setBusy("");
    }
  };
  if (!item || busy === "load")
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-500">
        <Loader2 className="mr-2 size-4 animate-spin" />
        {t("functionEditor.loading")}
      </div>
    );
  return (
    <section className="flex h-full min-h-0 flex-col">
      <header className="title-drag flex h-16 shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-950 px-5">
        <div className="title-no-drag flex items-center gap-3">
          <Button
            size="sm"
            variant="ghost"
            onClick={() => setScreen("functions")}
            aria-label={t("functionEditor.back")}
          >
            <ArrowLeft className="size-4" />
          </Button>
          <div>
            <Input
              value={item.name}
              onChange={(event) => update({ name: event.target.value })}
              className="h-7 w-64 text-sm font-semibold"
            />
            <p className="mt-1 text-xs text-zinc-600">
              {item.mode === "pure" ? t("functions.pure") : t("functions.impure")} ·{" "}
              {item.publishedRevision
                ? t("functionEditor.published", { version: item.publishedRevision })
                : t("functionEditor.draft")}
              {dirty ? ` · ${t("functionEditor.unsaved")}` : ""}
            </p>
          </div>
        </div>
        <div className="title-no-drag flex gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowLibrary((value) => !value)}
            aria-pressed={showLibrary}
          >
            <PanelLeft className="size-3.5" />
            {t("editor.library")}
          </Button>
          <Button
            variant="outline"
            size="sm"
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
          <Button
            size="sm"
            onClick={() => void save(true)}
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
      <div
        className="grid min-h-0 flex-1"
        style={{ gridTemplateColumns: `${showLibrary ? "246px " : ""}minmax(0,1fr) 330px` }}
      >
        {showLibrary ? (
          <BlueprintNodeLibrary
            definitions={filteredDefinitions}
            search={libraryQuery}
            onSearch={setLibraryQuery}
            onAdd={addNode}
            dragMime="application/neuropipe-function-node"
            preferenceKey={functionLibraryCollapsedCategoriesKey}
          />
        ) : null}
        <div ref={canvasRef} className="relative min-w-0">
          <ReactFlow
            className="select-none"
            nodes={nodes}
            edges={edges}
            nodeTypes={graphTypes}
            defaultEdgeOptions={defaultEdgeOptions}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            isValidConnection={isValidConnection}
            onConnect={(connection) => { onConnect(connection); setConnecting(undefined); }}
            onConnectStart={(_, parameters) => {
              if (reconnectRef.current || !parameters.nodeId) return;
              setConnecting({ source: parameters.nodeId, sourceHandle: parameters.handleId });
            }}
            onConnectEnd={(event) => {
              if (reconnectRef.current) return;
              const target = event.target as Element;
              if (connecting && target.classList.contains("react-flow__pane") && "clientX" in event) openMenu(event, undefined, undefined, connecting);
            }}
            onReconnectStart={onReconnectStart}
            onReconnect={onReconnect}
            onReconnectEnd={onReconnectEnd}
            onNodeClick={(_, node) => setSelectedID(node.id)}
            onNodeContextMenu={(event, node) => {
              setSelectedID(node.id);
              openMenu(event, node.id);
            }}
            onEdgeContextMenu={(event, edge) => openMenu(event, undefined, edge.id)}
            onPaneClick={() => {
              setSelectedID(undefined);
              setMenu(undefined);
            }}
            onPaneContextMenu={(event) => openMenu(event)}
            onInit={setFlow}
            onDrop={drop}
            onDragOver={(event) => {
              event.preventDefault();
              event.dataTransfer.dropEffect = "move";
            }}
            fitView
            snapToGrid={gridSnapMode === "on"}
            snapGrid={editorSnapGrid}
          >
            <Background color="#27272a" gap={20} size={1} />
            <Controls showInteractive={false} />
            <Panel position="top-left" className="!m-3 flex gap-1 rounded-md border border-zinc-700 bg-zinc-950 p-1">
              <Button size="sm" variant="ghost" onClick={autoLayout}>
                <LayoutGrid className="size-3.5" />
                {t("editor.layout")}
              </Button>
              <Tooltip content={gridSnapMode === "on" ? t("editor.snapOff") : t("editor.snapOn")} side="bottom">
                <Button size="sm" variant={gridSnapMode === "on" ? "secondary" : "ghost"} aria-pressed={gridSnapMode === "on"} onClick={() => setGridSnapMode(gridSnapMode === "on" ? "off" : "on")}>
                  <Magnet className="size-3.5" />
                  {t("editorActions.snap")}
                </Button>
              </Tooltip>
              <Button size="sm" variant="ghost" onClick={duplicateSelected} disabled={!selectedID}>
                <Copy className="size-3.5" />
                {t("editorActions.duplicate")}
              </Button>
              <Tooltip content={t("editorActions.delete")} side="bottom">
                <Button size="sm" variant="ghost" onClick={removeSelected} disabled={!selectedID} aria-label={t("editorActions.delete")}>
                  <Trash2 className="size-3.5 text-red-300" />
                </Button>
              </Tooltip>
            </Panel>
          </ReactFlow>
          {menu ? (
            <BlueprintContextMenu
              menu={menu}
              definitions={availableDefinitions}
              search={menuQuery}
              onSearch={setMenuQuery}
              onAdd={(definition) => {
                addFromMenu(definition);
              }}
              onDuplicate={() => {
                duplicateSelected();
                setMenu(undefined);
              }}
              onDelete={() => {
                removeSelected();
                setMenu(undefined);
              }}
              onClose={() => setMenu(undefined)}
              preferenceKey={functionLibraryCollapsedCategoriesKey}
              onRemoveEdge={removeEdge}
              onInsertReroute={insertReroute}
            />
          ) : null}
        </div>
        <aside className="muted-scroll overflow-y-auto border-l border-zinc-800 bg-zinc-950 p-4">
          <section className="space-y-3">
            <p className="text-[10px] font-semibold uppercase tracking-[.14em] text-zinc-600">
              {t("functionEditor.details")}
            </p>
            <Input
              value={item.category}
              onChange={(event) => update({ category: event.target.value })}
              placeholder={t("functionEditor.category")}
            />
            <textarea
              value={item.description}
              onChange={(event) => update({ description: event.target.value })}
              placeholder={t("functionEditor.description")}
              className="min-h-20 w-full rounded-md border border-zinc-700 bg-zinc-950 p-2 text-xs text-zinc-200 outline-none focus:border-zinc-500"
            />
            <p className="rounded-md border border-zinc-800 bg-zinc-900/50 px-2.5 py-2 text-xs text-zinc-400">
              {item.mode === "pure"
                ? t("functionEditor.pureDescription")
                : t("functionEditor.impureDescription")}
            </p>
          </section>
          <section className="mt-6">
            <p className="mb-2 text-[10px] font-semibold uppercase tracking-[.14em] text-zinc-600">
              {t("functionEditor.icon")}
            </p>
            <div className="flex items-center justify-between rounded-md border border-zinc-800 bg-zinc-900/40 px-2.5 py-2">
              <span className="flex size-8 items-center justify-center rounded-md border border-zinc-700" style={{ color: item.iconColor, backgroundColor: item.iconBackground }}>
                <LucideIcon name={item.icon} className="size-4" />
              </span>
              <span className="flex gap-1"><LucideIconPicker value={item.icon} label={t("functionEditor.icon")} iconColor={item.iconColor} iconBackground={item.iconBackground} onValueChange={(icon) => update({ icon })} /><IconAppearancePicker iconColor={item.iconColor} iconBackground={item.iconBackground} onIconColorChange={(iconColor) => update({ iconColor })} onIconBackgroundChange={(iconBackground) => update({ iconBackground })} /></span>
            </div>
          </section>
          {selectedDefinition?.fields.some(
            (field) =>
              field.kind === "field-outputs" ||
              field.kind === "object-fields" ||
              field.kind === "switch-cases",
          ) ? (
            <section className="mt-6 space-y-3">
              <p className="text-[10px] font-semibold uppercase tracking-[.14em] text-zinc-600">
                {t("editor.configureNode")}
              </p>
              {selectedDefinition.fields.map((field) => {
                const value =
                  selected?.data.config[field.name] ??
                  selectedDefinition.defaultConfig?.[field.name];
                if (field.kind === "field-outputs") {
                  return (
                    <FieldOutputsEditor
                      key={field.name}
                      value={value}
                      sourceFields={selectedSourceFields}
                      onChange={(next) => updateNodeConfig(field.name, next)}
                    />
                  );
                }
                if (field.kind === "object-fields") {
                  return (
                    <ObjectFieldsEditor
                      key={field.name}
                      value={value}
                      onChange={(next) => updateNodeConfig(field.name, next)}
                    />
                  );
                }
                if (field.kind === "switch-cases") {
                  return (
                    <BlueprintSwitchCasesEditor
                      key={field.name}
                      value={
                        selected?.data.config[field.name] === undefined &&
                        selected?.data.config.options !== undefined
                          ? undefined
                          : value
                      }
                      legacyOptions={selected?.data.config.options}
                      onChange={(next) => updateNodeConfig(field.name, next)}
                    />
                  );
                }
                return null;
              })}
            </section>
          ) : null}
          <PinList
            title={t("functionEditor.inputs")}
            pins={item.inputs}
            side="inputs"
            onAdd={() => addPin("inputs")}
            onUpdate={updatePin}
            onRemove={removePin}
          />
          <PinList
            title={t("functionEditor.outputs")}
            pins={item.outputs}
            side="outputs"
            onAdd={() => addPin("outputs")}
            onUpdate={updatePin}
            onRemove={removePin}
          />
          <Button
            variant="ghost"
            className="mt-6 w-full text-red-300 hover:text-red-200"
            onClick={() => void destroy()}
            disabled={busy !== ""}
          >
            <Trash2 className="size-3.5" />
            {t("functionEditor.deleteFunction")}
          </Button>
        </aside>
      </div>
    </section>
  );
}

function PinList({
  title,
  pins,
  side,
  onAdd,
  onUpdate,
  onRemove,
}: {
  title: string;
  pins: FunctionPin[];
  side: "inputs" | "outputs";
  onAdd: () => void;
  onUpdate: (
    side: "inputs" | "outputs",
    index: number,
    change: Partial<FunctionPin>,
  ) => void;
  onRemove: (side: "inputs" | "outputs", id: string) => void;
}) {
  const { t } = useTranslation();
  const options = dataTypes.map((value) => ({
    value,
    label: t(`editor.${value}`),
  }));
  return (
    <section className="mt-6">
      <div className="mb-2 flex items-center justify-between">
        <p className="text-[10px] font-semibold uppercase tracking-[.14em] text-zinc-600">
          {title}
        </p>
        <Button size="sm" variant="ghost" onClick={onAdd}>
          <Plus className="size-3" />
        </Button>
      </div>
      <div className="space-y-2">
        {pins.map((pin, index) => (
          <div
            key={pin.id}
            className="grid grid-cols-[minmax(0,1fr)_100px_28px] gap-1"
          >
            <Input
              value={pin.name}
              onChange={(event) =>
                onUpdate(side, index, { name: event.target.value })
              }
              aria-label={t("functionEditor.pinName", { title })}
            />
            <Select
              value={pin.dataType}
              onValueChange={(value) =>
                onUpdate(side, index, { dataType: value as DataType })
              }
              ariaLabel={t("functionEditor.pinType", { title })}
              options={options}
            />
            <Button
              size="sm"
              variant="ghost"
              aria-label={t("functionEditor.removePin", { name: pin.name || title })}
              onClick={() => onRemove(side, pin.id)}
            >
              <Trash2 className="size-3 text-red-300" />
            </Button>
          </div>
        ))}
      </div>
    </section>
  );
}

function hydrate(
  item: CustomFunction,
  definitions: NodeDefinition[],
): EditorNode[] {
  return (item.draftDefinition.nodes ?? []).map((node) => {
    const config = (node.data?.config ?? node.data ?? {}) as Record<
      string,
      unknown
    >;
    const nodeType = node.type ?? "";
    const definition = definitions.find((entry) => entry.type === nodeType);
    return {
      ...node,
      type: "functionNode",
      data: {
        type: nodeType,
        label: definition?.label ?? nodeType,
        inputs: graphInputs(nodeType, definition, item, config),
        outputs: graphOutputs(nodeType, definition, item, config),
        config,
      },
    };
  });
}

function graphInputs(
  type: string,
  definition: NodeDefinition | undefined,
  item: CustomFunction | undefined,
  config: Record<string, unknown>,
): NodePort[] {
  return boundaryPins(
    type,
    resolveConfigDrivenInputs(definition, config),
    item,
    "input",
  );
}

function graphOutputs(
  type: string,
  definition: NodeDefinition | undefined,
  item: CustomFunction | undefined,
  config: Record<string, unknown>,
): NodePort[] {
  return boundaryPins(
    type,
    resolveConfigDrivenOutputs(definition, config),
    item,
    "output",
  );
}

function isFunctionBoundary(type: string) {
  return (
    type === "function:entry" ||
    type === "function:return" ||
    type === "function:input" ||
    type === "function:output"
  );
}
function boundaryPins(
  type: string,
  pins: NodePort[],
  item: CustomFunction | undefined,
  direction: "input" | "output",
): NodePort[] {
  if (!item) return pins;
  if (type === "function:input" && direction === "output")
    return item.inputs.map((pin) => ({
      id: pin.id,
      label: pin.name,
      kind: "data" as const,
      direction: "output" as const,
      dataType: pin.dataType,
    }));
  if (type === "function:output" && direction === "input")
    return item.outputs.map((pin) => ({
      id: pin.id,
      label: pin.name,
      kind: "data" as const,
      direction: "input" as const,
      dataType: pin.dataType,
    }));
  if (type === "function:entry" && direction === "output")
    return [
      ...pins,
      ...item.inputs.map((pin) => ({
        id: pin.id,
        label: pin.name,
        kind: "data" as const,
        direction: "output" as const,
        dataType: pin.dataType,
      })),
    ];
  if (type === "function:return" && direction === "input")
    return [
      ...pins,
      ...item.outputs.map((pin) => ({
        id: pin.id,
        label: pin.name,
        kind: "data" as const,
        direction: "input" as const,
        dataType: pin.dataType,
      })),
    ];
  return pins;
}
