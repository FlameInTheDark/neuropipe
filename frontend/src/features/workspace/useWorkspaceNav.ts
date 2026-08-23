import { useCallback, useRef, useState } from "react";
import type { CustomFunction } from "@/lib/types";
import type { UiFunctionSummary } from "@/lib/adapters";
import type { UiPipeline } from "@/lib/adapters";

export type EditorKind = "pipeline" | "function";
export type RailId =
  | "board"
  | "chat"
  | "triggers"
  | "schedules"
  | "reports"
  | "runs"
  | "pipelines"
  | "functions"
  | "variables"
  | "databases"
  | "metrics"
  | "docs"
  | "settings";

/**
 * Owns "where am I" — the active rail section and whether a pipeline or
 * function editor is open. Loading happens through the callbacks provided
 * by `App` so this module stays free of backend concerns.
 */
export function useWorkspaceNav(options: {
  openPipeline: (id: string) => Promise<void>;
  openFunction: (id: string) => Promise<void>;
  closeEditor: () => void;
}) {
  const { openPipeline, openFunction, closeEditor } = options;
  const [rail, setRail] = useState<RailId>("pipelines");
  const [editingPipeline, setEditingPipeline] = useState<UiPipeline | null>(null);
  const [editingFunction, setEditingFunction] = useState<UiFunctionSummary | null>(null);
  const opening = useRef(false);

  const inEditor = editingPipeline !== null || editingFunction !== null;
  const editorKind: EditorKind | null = editingFunction ? "function" : editingPipeline ? "pipeline" : null;

  const doOpenPipeline = useCallback(
    async (p: UiPipeline) => {
      if (opening.current) return;
      opening.current = true;
      setEditingFunction(null);
      setEditingPipeline(p);
      try {
        await openPipeline(p.id);
      } finally {
        opening.current = false;
      }
    },
    [openPipeline],
  );

  const doOpenFunction = useCallback(
    async (fn: UiFunctionSummary) => {
      if (opening.current) return;
      opening.current = true;
      setEditingPipeline(null);
      setEditingFunction(fn);
      try {
        await openFunction(fn.id);
      } catch {
        setEditingFunction(null);
      } finally {
        opening.current = false;
      }
    },
    [openFunction],
  );

  /** Replaces the displayed summary after a save (name/version refresh). */
  const updateEditingPipeline = useCallback((patch: Partial<UiPipeline>) => {
    setEditingPipeline((p) => (p ? { ...p, ...patch } : p));
  }, []);

  const updateEditingFunction = useCallback((patch: Partial<UiFunctionSummary>) => {
    setEditingFunction((f) => (f ? { ...f, ...patch } : f));
  }, []);

  const doCloseEditor = useCallback(() => {
    closeEditor();
    const wasFunction = editingFunction !== null;
    setEditingPipeline(null);
    setEditingFunction(null);
    setRail(wasFunction ? "functions" : "pipelines");
  }, [closeEditor, editingFunction]);

  const goto = useCallback(
    (id: string) => {
      closeEditor();
      setEditingPipeline(null);
      setEditingFunction(null);
      setRail(id as RailId);
    },
    [closeEditor],
  );

  return {
    rail, setRail,
    editingPipeline, updateEditingPipeline,
    editingFunction, updateEditingFunction,
    inEditor, editorKind,
    openPipeline: doOpenPipeline,
    openFunction: doOpenFunction,
    closeEditor: doCloseEditor,
    goto,
  };
}

export type NavApi = ReturnType<typeof useWorkspaceNav>;
export type { CustomFunction };
