import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import { ask } from "@/stores/confirmation";
import i18n from "@/i18n";
import { desktop } from "@/lib/bridge";
import type { Database, Pipeline, TwitchIdentity, DiscordIdentity, TelegramIdentity } from "@/lib/types";
import { pipelineFromBackend } from "@/lib/adapters";
import { TopBar } from "./components/TopBar";
import { IconRail, NAV_ALL } from "./components/IconRail";
import { StatusBar } from "./components/StatusBar";
import { CommandPalette, type Command } from "./components/CommandPalette";
import { ContextMenuProvider } from "./components/ContextMenu";
import { DialogHosts } from "./components/DialogHosts";
import { Toaster } from "./components/layout/Toaster";
import { EmptyState } from "./components/ViewShell";
import type { View } from "./components/Canvas";
import { PipelineEditor } from "./features/graph/PipelineEditor";
import { useGraphEditor } from "./features/graph/useGraphEditor";
import {
  buildCommentMenu,
  buildEdgeMenu,
  buildGroupMenu,
  buildMultiMenu,
  buildNodeMenu,
  buildPortMenu,
} from "./features/graph/editor-menus";
import { useWorkspace } from "./features/workspace/useWorkspace";
import { useWorkspaceNav } from "./features/workspace/useWorkspaceNav";
import { renderView } from "./features/workspace/view-registry";
import { DocsDialog } from "./views/DocsView";
import { useToast } from "./hooks/useToast";
import { useHotkeys } from "./hooks/useHotkeys";
import { NODE_W } from "./data/graph";

const RAIL_W = { collapsed: 52, expanded: 208 };
const CHROME = { topBar: 44, statusBar: 26 };

export default function App() {
  const { t } = useTranslation();

  /* ---------- cross-cutting ---------- */
  const { toast, notify } = useToast();
  const workspace = useWorkspace(notify);

  /* editor canvas viewport — declared before the editor hook so loads can
     push the saved pan/zoom straight into the rendered view */
  const [view, setView] = useState<View>({ x: 40, y: 40, z: 1 });
  const graph = useGraphEditor({
    notify,
    definitionIndex: workspace.definitionIndex,
    runningMap: workspace.running,
  });

  /* ---------- shell state ---------- */
  const [railExpanded, setRailExpanded] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [savedAt, setSavedAt] = useState<string | null>(null);
  const [busyAction, setBusyAction] = useState<string | null>(null);

  /* ---------- editor viewport / panels ---------- */
  const [snap, setSnap] = useState(true);
  const [leftOpen, setLeftOpen] = useState(true);
  const [rightOpen, setRightOpen] = useState(true);
  const [leftWidth, setLeftWidth] = useState(268);
  const [rightWidth, setRightWidth] = useState(320);
  const fitRef = useRef<(() => void) | null>(null);

  /* geometry must exist before any callback below closes over it */
  const railWidth = railExpanded ? RAIL_W.expanded : RAIL_W.collapsed;
  const leftOffset = 12 + railWidth + 12;

  useEffect(() => {
    graph.viewRef.current = view;
  }, [graph, view]);

  /* ---------- open/close editors ---------- */

  const openPipeline = useCallback(
    async (id: string) => {
      try {
        const full = await desktop.getPipeline(id);
        await graph.loadPipeline(full);
      } catch {
        notify(t("editor.loadFailed"), "AlertTriangle");
        throw new Error("load failed");
      }
    },
    [graph, notify, t],
  );

  const openFunction = useCallback(
    async (id: string) => {
      const full = await workspace.getFunction(id);
      await graph.loadFunction(full);
    },
    [graph, workspace],
  );

  const closeEditor = useCallback(() => graph.close(), [graph]);

  /** Asks for confirmation before leaving the editor with unsaved changes. */
  const confirmLeave = useCallback(async (): Promise<boolean> => {
    if (!graph.dirty) return true;
    return ask({
      title: t("editor.unsavedTitle"),
      description: t("editor.unsavedDescription"),
      confirmLabel: t("editor.unsavedConfirm"),
      danger: true,
    });
  }, [graph.dirty, t]);

  const nav = useWorkspaceNav({ openPipeline, openFunction, closeEditor });

  /** Guarded back: confirms before closing when the draft is dirty. */
  const handleBack = useCallback(() => {
    if (nav.inEditor && graph.dirty) {
      void confirmLeave().then((ok) => { if (ok) nav.closeEditor(); });
      return;
    }
    nav.closeEditor();
  }, [nav, graph.dirty, confirmLeave]);

  /** Guarded rail navigation: confirms before leaving a dirty editor. */
  const guardedGoto = useCallback((id: string) => {
    if (nav.inEditor && graph.dirty) {
      void confirmLeave().then((ok) => { if (ok) nav.goto(id); });
      return;
    }
    nav.goto(id);
  }, [nav, graph.dirty, confirmLeave]);

  /* freshly opened graphs fit themselves into the visible canvas area,
     like pressing "Fit graph to view" */
  const editorTargetId = nav.editingPipeline?.id ?? nav.editingFunction?.id ?? null;
  useEffect(() => {
    if (!nav.inEditor || !editorTargetId) return;
    const frame = requestAnimationFrame(() => fitRef.current?.());
    return () => cancelAnimationFrame(frame);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editorTargetId, nav.inEditor]);

  useEffect(() => {
    if (!nav.inEditor) setSavedAt(null);
  }, [nav.inEditor]);

  /* ---------- backend events ---------- */

  useEffect(() => {
    const offs = [
      Events.On("app.open.settings", () => nav.goto("settings")),
      Events.On("dialog.input.request", (e: unknown) => {
        void import("@/stores/dialogs").then((m) => m.dispatchInputDialogRequest(extractPayload(e) as never));
      }),
      Events.On("dialog.input.cancel", (e: unknown) => {
        const payload = extractPayload(e);
        const id = typeof payload === "string" ? payload : String((payload as { id?: string })?.id ?? "");
        void import("@/stores/dialogs").then((m) => m.cancelInputDialog(id));
      }),
      Events.On("dialog.form.request", (e: unknown) => {
        void import("@/stores/dialogs").then((m) => m.dispatchFormDialogRequest(extractPayload(e) as never));
      }),
      Events.On("dialog.form.cancel", (e: unknown) => {
        const payload = extractPayload(e);
        const id = typeof payload === "string" ? payload : String((payload as { id?: string })?.id ?? "");
        void import("@/stores/dialogs").then((m) => m.cancelFormDialog(id));
      }),
      Events.On("app.update.available", (e: unknown) => {
        workspace.setUpdate(extractPayload(e) as never);
      }),
      Events.On("reports.updated", () => void workspace.refreshReports()),
    ];
    return () => offs.forEach((off) => off());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace.refreshReports]);

  /* ---------- per-page data refresh ---------- */

  /* Anything outside the app (assistant authoring tools, executor runs,
     API clients) can mutate workspace data while the user is elsewhere.
     Re-reading on section entry keeps lists honest without polling. */
  useEffect(() => {
    if (nav.inEditor) return;
    switch (nav.rail) {
      case "pipelines":
        void workspace.refresh();
        break;
      case "functions":
        void workspace.refreshFunctions();
        break;
      case "variables":
        void workspace.refreshVariables();
        break;
      case "reports":
        void workspace.refreshReports();
        break;
      case "board":
      case "triggers":
      case "schedules":
      case "runs":
        void workspace.refreshTriggers();
        break;
      case "settings":
        // integration pages render settings-backed identities and trigger
        // bindings; both mutate server-side while the user is elsewhere
        void workspace.refreshSettings();
        void workspace.refreshTriggers();
        break;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nav.rail, nav.inEditor]);

  /* ---------- tray menu labels follow the locale ---------- */

  useEffect(() => {
    void desktop
      .configureTrayMenu({
        show: t("tray.show"),
        settings: t("tray.settings"),
        hide: t("tray.hide"),
        close: t("tray.close"),
      })
      .catch(() => undefined);
  }, [t, i18n.resolvedLanguage]);

  /* ---------- editor actions ---------- */

  const save = useCallback(async () => {
    if (!nav.inEditor || busyAction) return;
    if (nav.editorKind === "pipeline" && graph.pipeline && nav.editingPipeline) {
      setBusyAction("save");
      try {
        const draftDefinition = graph.serialize(view);
        const saved: Pipeline = await desktop.savePipeline({
          ...graph.pipeline,
          name: nav.editingPipeline.name,
          description: nav.editingPipeline.desc ?? graph.pipeline.description,
          draftDefinition,
        });
        graph.setDirty(false);
        nav.updateEditingPipeline(pipelineFromBackend(saved));
        setSavedAt(new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }));
        notify(t("editor.draftSaved"), "Save");
      } catch {
        notify(t("editor.saveFailed"), "AlertTriangle");
      } finally {
        setBusyAction(null);
      }
      return;
    }
    if (nav.editorKind === "function" && graph.fn && nav.editingFunction) {
      setBusyAction("save");
      try {
        const item = graph.serializeFunction(view);
        if (!item) return;
        item.name = nav.editingFunction.name;
        item.description = nav.editingFunction.desc;
        const saved = await desktop.saveFunction(item);
        graph.setDirty(false);
        nav.updateEditingFunction({ name: saved.name });
        setSavedAt(new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }));
        notify(t("editor.draftSaved"), "Save");
      } catch {
        notify(t("functionEditor.saveFailed"), "AlertTriangle");
      } finally {
        setBusyAction(null);
      }
    }
  }, [nav, graph, view, busyAction, notify, t]);

  const publish = useCallback(async () => {
    if (!nav.inEditor || busyAction) return;
    setBusyAction("publish");
    try {
      if (nav.editorKind === "pipeline" && graph.pipeline && nav.editingPipeline) {
        const draftDefinition = graph.serialize(view);
        const saved = await desktop.savePipeline({
          ...graph.pipeline,
          name: nav.editingPipeline.name,
          draftDefinition,
        });
        const published = await desktop.publishPipeline(saved);
        graph.setDirty(false);
        await graph.loadPipeline(published);
        nav.updateEditingPipeline(pipelineFromBackend(published));
        // publishing creates/updates trigger bindings; keep trigger lists
        // (board, triggers view, integration pages) honest immediately
        void workspace.refreshTriggers();
        notify(t("editor.published"), "UploadCloud");
      } else if (nav.editorKind === "function" && graph.fn && nav.editingFunction) {
        const item = graph.serializeFunction(view);
        if (!item) return;
        item.name = nav.editingFunction.name;
        const saved = await desktop.saveFunction(item);
        const published = await desktop.publishFunction(saved);
        graph.setDirty(false);
        await graph.loadFunction(published);
        nav.updateEditingFunction({ name: published.name });
        notify(t("editor.published"), "UploadCloud");
      }
    } catch {
      notify(t("editor.publishFailed"), "AlertTriangle");
    } finally {
      setBusyAction(null);
    }
  }, [nav, graph, view, busyAction, notify, t]);

  const rename = useCallback(
    (name: string, description?: string) => {
      const next = name.trim();
      if (!next) return;
      const nextDescription = (description ?? "").trim();
      if (nav.editingFunction) {
        const nameChanged = next !== nav.editingFunction.name;
        const descChanged = nextDescription !== nav.editingFunction.desc;
        if (!nameChanged && !descChanged) return;
        nav.updateEditingFunction({ name: next, desc: nextDescription });
        graph.setDirty(true);
        return;
      }
      if (!nav.editingPipeline) return;
      const nameChanged = next !== nav.editingPipeline.name;
      const descChanged = nextDescription !== (nav.editingPipeline.desc ?? "");
      if (!nameChanged && !descChanged) return;
      nav.updateEditingPipeline({ name: next, desc: nextDescription });
      graph.setDirty(true);
    },
    [nav, graph],
  );

  /** Places a library node in the middle of the visible canvas gap. */
  const addNodeCentered = useCallback(
    (item: Parameters<typeof graph.addNode>[0], group: string) => {
      const leftGap = leftOpen ? leftOffset + leftWidth + 12 : leftOffset;
      const rightGap = rightOpen ? rightWidth + 24 : 12;
      const cx = leftGap + (window.innerWidth - leftGap - rightGap) / 2;
      const cy = (window.innerHeight - CHROME.topBar - CHROME.statusBar) / 2;
      graph.addNode(item, group, {
        x: (cx - view.x) / view.z - NODE_W / 2,
        y: (cy - view.y) / view.z - 60,
      });
    },
    [graph, leftOpen, rightOpen, leftOffset, leftWidth, rightWidth, view],
  );

  /* ---------- context menus ---------- */

  const menus = useMemo(
    () => ({
      node: (id: string) =>
        buildNodeMenu(graph.nodes.find((n) => n.id === id), t, {
          duplicate: graph.duplicateSelected,
          copy: graph.copyNode,
          clearPortLinks: graph.removeEdgesFor,
          remove: graph.deleteSelected,
        }),
      multi: (count: number) =>
        buildMultiMenu(
          {
            count,
            group: () => void graph.groupSelection(),
            duplicate: graph.duplicateSelected,
            align: graph.alignSelection,
            distribute: graph.distributeSelection,
            remove: graph.deleteSelected,
            clear: graph.clearSelection,
          },
          t,
        ),
      group: (id: string) =>
        buildGroupMenu(
          graph.groups.find((g) => g.id === id),
          {
            selectMembers: graph.selectGroupMembers,
            beginRename: graph.beginRenameGroup,
            setColor: graph.setGroupColor,
            ungroup: graph.ungroup,
          },
          t,
        ),
      comment: (id: string) =>
        buildCommentMenu(
          graph.comments.find((c) => c.id === id),
          {
            beginRename: graph.setRenamingCommentId,
            setColor: graph.setCommentColor,
            remove: graph.removeComment,
          },
          t,
        ),
      edge: (id: string, at: { x: number; y: number }) =>
        buildEdgeMenu(id, graph.removeEdge, t, (eid, atPoint) => graph.insertReroute(eid, atPoint), at),
      port: (nodeId: string, portId: string) => buildPortMenu(nodeId, portId, graph.removeEdgesFor, t),
    }),
    [graph, t],
  );

  /* ---------- shortcuts ---------- */

  useHotkeys(
    useMemo(
      () => [
        { key: "k", mod: true, run: () => setPaletteOpen((p) => !p) },
        { key: "Escape", run: () => setPaletteOpen(false) },
        { key: "s", mod: true, when: nav.inEditor, run: () => void save() },
        { key: "b", mod: true, when: nav.inEditor, run: () => setLeftOpen((v) => !v) },
        { key: "Enter", mod: true, when: nav.inEditor, run: () => void graph.run() },
        { key: "d", mod: true, when: nav.inEditor, run: graph.duplicateSelected },
        { key: "c", mod: true, skipWhenTyping: true, when: nav.inEditor, run: graph.copySelected },
        { key: "v", mod: true, skipWhenTyping: true, when: nav.inEditor, run: graph.pasteClipboard },
        { key: "Backspace", skipWhenTyping: true, when: nav.inEditor, run: graph.deleteSelected },
        { key: "Delete", skipWhenTyping: true, when: nav.inEditor, run: graph.deleteSelected },
      ],
      [nav.inEditor, save, graph.run, graph.duplicateSelected, graph.copySelected, graph.pasteClipboard, graph.deleteSelected],
    ),
  );

  /* ---------- command palette ---------- */

  const commands: Command[] = useMemo(() => {
    const navigate: Command[] = NAV_ALL.map((n) => ({
      id: `nav-${n.id}`,
      label: `${t("palette.goTo")} ${t(n.labelKey)}`,
      icon: n.icon,
      group: t("palette.groupNavigate"),
      run: () => guardedGoto(n.id),
    }));

    if (nav.inEditor) {
      /* every library entry is placeable straight into the viewport center */
      const nodes: Command[] = workspace.library.flatMap((cat) =>
        cat.items.map((item) => ({
          id: `node-${item.functionId ?? item.type}`,
          label: item.name,
          icon: item.icon,
          group: t("palette.groupNodes"),
          hint: cat.name,
          run: () => addNodeCentered(item, cat.name),
        })),
      );
      const editor: Command[] = [
        { id: "run", label: t("editor.runDraft"), icon: "Play", group: t("palette.groupEditor"), hint: "⌘↵", run: () => void graph.run() },
        { id: "save", label: t("editor.menuSaveDraft"), icon: "Save", group: t("palette.groupEditor"), hint: "⌘S", run: () => void save() },
        { id: "fit", label: t("editor.fitGraph"), icon: "Maximize2", group: t("palette.groupEditor"), run: () => fitRef.current?.() },
        {
          id: "snap",
          label: snap ? t("editor.snapOff") : t("editor.snapOn"),
          icon: "Magnet",
          group: t("palette.groupEditor"),
          run: () => setSnap(!snap),
        },
        { id: "dup", label: t("editor.duplicateNode"), icon: "Copy", group: t("palette.groupEditor"), hint: "⌘D", run: graph.duplicateSelected },
        { id: "del", label: t("editor.deleteNode"), icon: "Trash2", group: t("palette.groupEditor"), hint: "⌫", run: graph.deleteSelected },
        { id: "close", label: t("editor.closeEditor"), icon: "ArrowLeft", group: t("palette.groupEditor"), run: nav.closeEditor },
      ];
      return [...nodes, ...editor, ...navigate];
    }

    /* launcher mode: every pipeline and function is directly openable */
    const pipelines: Command[] = workspace.pipelines.map((p) => ({
      id: `pipeline-${p.id}`,
      label: p.name,
      icon: p.icon || "Cable",
      group: t("palette.groupPipelines"),
      hint: p.version || undefined,
      // must go through nav so the editor actually opens, not just loads
      run: () => {
        nav.openPipeline(p).catch(() => undefined);
      },
    }));
    const functions: Command[] = workspace.functions.map((f) => ({
      id: `function-${f.id}`,
      label: f.name,
      icon: f.icon || "Braces",
      group: t("palette.groupFunctions"),
      hint: f.publishedRevision > 0 ? `v${f.publishedRevision}` : undefined,
      run: () => {
        void nav.openFunction(f);
      },
    }));
    return [...pipelines, ...functions, ...navigate];
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nav, graph.run, graph.duplicateSelected, graph.deleteSelected, save, snap, t, guardedGoto, workspace.pipelines, workspace.functions, workspace.library, addNodeCentered]);

  /* ---------- documentation overlay (editor-safe) ---------- */

  /** Opens the docs modal, deep-linked to a node's own page when possible.
   *  Deliberately never navigates: leaving the editor could discard
   *  unsaved graph changes. Falls back to the generic docs entry. */
  const [docsTarget, setDocsTarget] = useState<{ id?: string; anchor?: string } | null>(null);
  const openDocs = useCallback((nodeType?: string) => {
    if (!nodeType) {
      setDocsTarget({});
      return;
    }
    void desktop
      .getDocumentationForNode(nodeType)
      .then((ref) =>
        setDocsTarget(ref?.documentId ? { id: ref.documentId, anchor: ref.anchor } : {}),
      )
      .catch(() => setDocsTarget({}));
  }, []);

  /* ---------- inspector option sources ---------- */

  const [secrets, setSecrets] = useState<string[]>([]);
  const [databases, setDatabases] = useState<Database[]>([]);
  useEffect(() => {
    if (!nav.inEditor) return;
    void desktop.listSecrets().then((list) => setSecrets(list.map((s) => s.name))).catch(() => undefined);
    void desktop.listDatabases().then(setDatabases).catch(() => undefined);
  }, [nav.inEditor]);

  const identities: TwitchIdentity[] = useMemo(
    () => workspace.settings?.twitch.identities ?? [],
    [workspace.settings],
  );
  const discordIdentities: DiscordIdentity[] = useMemo(
    () => workspace.settings?.discord?.identities ?? [],
    [workspace.settings],
  );
  const telegramIdentities: TelegramIdentity[] = useMemo(
    () => workspace.settings?.telegram?.identities ?? [],
    [workspace.settings],
  );

  const viewTitle = t(NAV_ALL.find((n) => n.id === nav.rail)?.labelKey ?? "app.title");
  const activeRuns = Object.values(workspace.running).filter(Boolean).length;

  /* ---------- global loading / error ---------- */

  if (workspace.loading) {
    return (
      <div className="flex h-screen items-center justify-center bg-ink-1000 text-ink-300">
        <span className="flex items-center gap-2 text-[13px]">
          <span className="h-4 w-4 animate-spin rounded-full border-2 border-ink-600 border-t-ink-100" />
          {t("app.opening")}
        </span>
      </div>
    );
  }

  if (workspace.error && !workspace.settings) {
    return (
      <div className="flex h-screen items-center justify-center bg-ink-1000">
        <div className="max-w-[420px] text-center">
          <EmptyState icon="AlertTriangle" title={t("common.unavailable")} hint={workspace.error} />
        </div>
      </div>
    );
  }

  /* ---------- render ---------- */

  return (
    <ContextMenuProvider>
      <div className="flex h-screen flex-col overflow-hidden bg-ink-1000 text-ink-100">
        <TopBar
          inEditor={nav.inEditor}
          viewTitle={viewTitle}
          parentTitle={nav.editorKind === "function" ? t("nav.functions") : t("nav.pipelines")}
          pipelineName={nav.editingFunction?.name ?? nav.editingPipeline?.name}
          executorName={nav.editingPipeline?.executorName}
          description={nav.editingFunction?.desc ?? nav.editingPipeline?.desc}
          descriptionLabel={t("editor.descriptionLabel")}
          version={
            nav.editorKind === "function"
              ? nav.editingFunction?.publishedRevision
                ? `v${nav.editingFunction.publishedRevision}`
                : t("functions.draft")
              : nav.editingPipeline?.version
          }
          busy={busyAction}
          onRename={rename}
          dirty={graph.dirty}
          running={graph.running}
          onBack={handleBack}
          onSave={() => void save()}
          onRun={() => void graph.run()}
          onStop={() => void graph.stop()}
          onPublish={() => void publish()}
          onCommand={() => setPaletteOpen(true)}
          leftOpen={leftOpen}
          rightOpen={rightOpen}
          toggleLeft={() => setLeftOpen((v) => !v)}
          toggleRight={() => setRightOpen((v) => !v)}
        />

        <div className="relative flex min-h-0 flex-1">
          <main className="relative min-w-0 flex-1 overflow-hidden">
            {nav.inEditor ? (
              graph.loadError ? (
                <div style={{ paddingLeft: leftOffset }} className="h-full p-3 pl-0">
                  <div className="flex h-full w-full items-center justify-center rounded-xl border border-ink-700 bg-ink-900/70">
                    <EmptyState
                      icon="AlertTriangle"
                      title={t("editor.legacyTitle")}
                      hint={t("editor.legacyDescription")}
                    />
                  </div>
                </div>
              ) : (
                <PipelineEditor
                  graph={{ ...graph, addNode: graph.addNode }}
                  panels={{
                    leftOpen,
                    rightOpen,
                    leftWidth,
                    rightWidth,
                    leftOffset,
                    setLeftWidth,
                    setRightWidth,
                  }}
                  view={view}
                  setView={setView}
                  snap={snap}
                  setSnap={setSnap}
                  registerFit={(fn) => (fitRef.current = fn)}
                  menus={menus}
                  onLibraryAdd={addNodeCentered}
                  library={workspace.library}
                  editorApi={{
                    pipelines: workspace.pipelines,
                    secrets,
                    databases,
                    identities,
                    discordIdentities,
                    telegramIdentities,
                    validateJavaScript: desktop.validateJavaScript,
                    generateCode: desktop.generateCode,
                    inspectDatabase: desktop.inspectDatabase,
                    debugDatabase: desktop.debugDatabase,
                    openDocs: openDocs,
                    executions: graph.executions,
                    onLoadExecution: graph.loadExecution,
                  }}
                />
              )
            ) : (
              <div
                style={{ paddingLeft: leftOffset }}
                className="h-full p-3 pl-0 transition-[padding] duration-[220ms] ease-[cubic-bezier(0.4,0,0.2,1)]"
              >
                <div className="h-full w-full overflow-hidden rounded-xl border border-ink-700 bg-ink-900/70 shadow-[0_24px_60px_-24px_rgba(0,0,0,0.9)]">
                  {renderView(nav.rail, { workspace, nav })}
                </div>
              </div>
            )}

            <IconRail
              active={nav.inEditor ? (nav.editorKind === "function" ? "functions" : "pipelines") : nav.rail}
              expanded={railExpanded}
              onSelect={guardedGoto}
              onToggle={() => setRailExpanded((v) => !v)}
            />
          </main>
        </div>

        <StatusBar
          inEditor={nav.inEditor}
          nodes={graph.nodes.length}
          edges={graph.edges.length}
          zoom={view.z}
          snap={snap}
          running={graph.running}
          saved={savedAt}
          selected={graph.selectedId}
          activeRuns={activeRuns}
          contentDirectory={workspace.settings?.contentDirectory}
          update={workspace.update}
          onFit={() => fitRef.current?.()}
        />

        <Toaster toast={toast} />
        <CommandPalette open={paletteOpen} commands={commands} onClose={() => setPaletteOpen(false)} />
        {docsTarget && (
          <DocsDialog
            documentID={docsTarget.id ?? null}
            anchor={docsTarget.anchor ?? null}
            onClose={() => setDocsTarget(null)}
          />
        )}
        <DialogHosts />
      </div>
    </ContextMenuProvider>
  );
}

/** Wails v3 beta wraps payloads in `{ data: … }`; unwrap defensively. */
export function extractPayload(event: unknown): unknown {
  if (event && typeof event === "object" && "data" in event) {
    return (event as { data?: unknown }).data ?? event;
  }
  return event;
}




