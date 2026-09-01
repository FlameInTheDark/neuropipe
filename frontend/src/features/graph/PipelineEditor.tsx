import { Canvas, type View } from "../../components/Canvas";
import { Inspector } from "../../components/Inspector";
import { LibraryPanel } from "../../components/LibraryPanel";
import { FloatPanel } from "../../components/layout/FloatPanel";
import type { MenuItem } from "../../components/ContextMenu";
import type { Database, TwitchIdentity, DiscordIdentity, TelegramIdentity, CodeGenerationRequest, CodeGenerationResponse, DatabaseSchema, Execution, SQLDebugRequest, SQLResult, Storage } from "@/lib/types";
import type { EditorComment, Edge, GraphNode, GroupColor, LibraryCategory, LogEntry, NodeGroup, Port, PortKind } from "@/types";

const PANEL_BOUNDS = { minLeft: 220, maxLeft: 420, minRight: 280, maxRight: 520 };
const clamp = (v: number, min: number, max: number) => Math.max(min, Math.min(max, v));

export interface PanelState {
  leftOpen: boolean;
  rightOpen: boolean;
  leftWidth: number;
  rightWidth: number;
  /** distance from the viewport edge to where the library panel starts */
  leftOffset: number;
  setLeftWidth: React.Dispatch<React.SetStateAction<number>>;
  setRightWidth: React.Dispatch<React.SetStateAction<number>>;
}

/** Backend-backed services the inspector panels need. */
export interface EditorApi {
  secrets: string[];
  /** published pipeline summaries for Run Pipeline / List Pipelines nodes */
  pipelines: import("@/lib/adapters").UiPipeline[];
  databases: Database[];
  storages: Storage[];
  identities: TwitchIdentity[];
  discordIdentities: DiscordIdentity[];
  telegramIdentities: TelegramIdentity[];
  /** configured LLM providers for AI node provider/model pickers */
  providers: import("@/lib/types").ProviderConfig[];
  defaultProviderId: string;
  validateJavaScript: (code: string) => Promise<void>;
  generateCode: (request: CodeGenerationRequest) => Promise<CodeGenerationResponse>;
  inspectDatabase: (id: string) => Promise<DatabaseSchema>;
  debugDatabase: (request: SQLDebugRequest) => Promise<SQLResult>;
  /** opens documentation, optionally for a node type — never navigates away from the editor */
  openDocs: (nodeType?: string) => void;
  executions: Execution[];
  onLoadExecution: (execution: Execution) => void;
}

/**
 * The graph editing workspace: canvas plus its two floating panels.
 */
export function PipelineEditor({
  graph,
  panels,
  view,
  setView,
  snap,
  setSnap,
  registerFit,
  menus,
  onLibraryAdd,
  library,
  editorApi,
}: {
  onLibraryAdd: (item: import("@/types").LibraryItem, group: string) => void;
  graph: {
    nodes: GraphNode[];
    edges: Edge[];
    log: LogEntry[];
    selected: GraphNode | null;
    selectedId: string | null;
    setSelectedId: (id: string | null) => void;
    selectedIds: Set<string>;
    groups: NodeGroup[];
    selectedGroupId: string | null;
    setSelectedGroupId: (id: string | null) => void;
    renamingGroupId: string | null;
renamingCommentId: string | null;
    comments: EditorComment[];
    selectedCommentId: string | null;
    setSelectedCommentId: (id: string | null) => void;
    setRenamingCommentId: (id: string | null) => void;
    addComment: (at: { x: number; y: number }) => void;
    renameComment: (id: string, text: string) => void;
    setCommentColor: (id: string, color: GroupColor) => void;
    resizeComment: (id: string, rect: { x: number; y: number; w: number; h: number }) => void;
    moveComment: (id: string, x: number, y: number) => void;
    removeComment: (id: string) => void;
    selectOnly: (id: string | null) => void;
    clearSelection: () => void;
    toggleSelect: (id: string) => void;
    selectMarquee: (
      rect: { x: number; y: number; w: number; h: number },
      mode: "replace" | "add" | "subtract",
    ) => void;
    moveNode: (id: string, x: number, y: number) => void;
    moveNodes: (positions: Record<string, { x: number; y: number }>) => void;
    moveGroup: (
      id: string,
      x: number,
      y: number,
      memberPositions: Record<string, { x: number; y: number }>,
    ) => void;
    resizeGroup: (id: string, rect: { x: number; y: number; w: number; h: number }) => void;
    renameGroup: (id: string, title: string) => void;
    alignSelection: (axis: "left" | "right" | "top" | "bottom" | "hcenter" | "vcenter") => void;
    distributeSelection: (axis: "h" | "v") => void;
    connect: (
      from: { node: string; port: string },
      to: { node: string; port: string },
      kind: PortKind,
    ) => void;
    removeEdge: (id: string) => void;
    insertReroute: (edgeId: string, at: { x: number; y: number }) => void;
    duplicateSelected: () => void;
    deleteSelected: () => void;
    addNode: (item: import("@/types").LibraryItem, group: string, at: { x: number; y: number }) => void;
    updateField: (key: string, value: unknown) => void;
    updateNodePorts: (nodeId: string, inputs: Port[], outputs: Port[]) => void;
    setFunctionKind: (kind: "pure" | "impure" | "tool") => void;
    mode: "pipeline" | "function" | null;
  };
  panels: PanelState;
  view: View;
  setView: (v: View | ((v: View) => View)) => void;
  snap: boolean;
  setSnap: (v: boolean) => void;
  registerFit: (fn: () => void) => void;
  menus: {
    node: (id: string) => MenuItem[];
    multi: (count: number) => MenuItem[];
    group: (id: string) => MenuItem[];
    comment: (id: string) => MenuItem[];
    edge: (id: string, at: { x: number; y: number }) => MenuItem[];
    port: (nodeId: string, portId: string) => MenuItem[];
  };
  library: LibraryCategory[];
  editorApi: EditorApi;
}) {
  const { leftOpen, rightOpen, leftWidth, rightWidth, leftOffset, setLeftWidth, setRightWidth } = panels;

  return (
    <div className="relative flex h-full w-full flex-col">
      <Canvas
        nodes={graph.nodes}
        edges={graph.edges}
        library={library}
        selectedId={graph.selectedId}
        selectedIds={graph.selectedIds}
        groups={graph.groups}
        selectedGroupId={graph.selectedGroupId}
        renamingGroupId={graph.renamingGroupId}
        comments={graph.comments}
        selectedCommentId={graph.selectedCommentId}
        setSelectedCommentId={graph.setSelectedCommentId}
        renamingCommentId={graph.renamingCommentId}
        setRenamingCommentId={graph.setRenamingCommentId}
        onAddComment={graph.addComment}
        onRenameComment={graph.renameComment}
        onResizeComment={graph.resizeComment}
        onMoveComment={graph.moveComment}
        onRemoveComment={graph.removeComment}
        commentCtx={menus.comment}
        view={view}
        snap={snap}
        onSelect={graph.selectOnly}
        onSelectGroup={graph.setSelectedGroupId}
        onRenameGroup={graph.renameGroup}
        onMoveGroup={graph.moveGroup}
        onResizeGroup={graph.resizeGroup}
        onMoveMany={graph.moveNodes}
        onToggleSelect={graph.toggleSelect}
        onClearSelection={graph.clearSelection}
        selectMarquee={graph.selectMarquee}
        onMove={graph.moveNode}
        onConnect={graph.connect}
        onRemoveEdge={graph.removeEdge}
        setView={setView}
        setSnap={setSnap}
        onDuplicate={graph.duplicateSelected}
        onDelete={graph.deleteSelected}
        onAddNode={graph.addNode}
        registerFit={registerFit}
        leftInset={leftOpen ? leftOffset + leftWidth + 12 : leftOffset}
        rightInset={rightOpen ? rightWidth + 24 : 12}
        nodeCtx={menus.node}
        multiCtx={() => menus.multi(graph.selectedIds.size)}
        groupCtx={menus.group}
        edgeCtx={menus.edge}
        portCtx={menus.port}
      />

      <FloatPanel
        open={leftOpen}
        side="left"
        width={leftWidth}
        offset={leftOffset}
        onResize={(dx) => setLeftWidth((w) => clamp(w + dx, PANEL_BOUNDS.minLeft, PANEL_BOUNDS.maxLeft))}
      >
        <LibraryPanel library={library} onAdd={onLibraryAdd} />
      </FloatPanel>

      <FloatPanel
        open={rightOpen}
        side="right"
        width={rightWidth}
        onResize={(dx) => setRightWidth((w) => clamp(w - dx, PANEL_BOUNDS.minRight, PANEL_BOUNDS.maxRight))}
      >
        <Inspector
          node={graph.selected}
          log={graph.log}
          api={editorApi}
          onChange={graph.updateField}
          onPortsChange={graph.updateNodePorts}
          onFunctionKindChange={graph.setFunctionKind}
        />
      </FloatPanel>
    </div>
  );
}

/** Colour choices for group frames — exported for the context menu builder. */
export const GROUP_COLOR_TOKENS: GroupColor[] = ["slate", "violet", "emerald", "amber", "sky", "rose"];


