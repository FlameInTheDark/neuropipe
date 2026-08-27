import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import i18n from "@/i18n";
import { desktop } from "@/lib/bridge";
import type {
  CustomFunction,
  CreateFunctionRequest,
  Execution,
  GlobalVariableSummary,
  Pipeline,
  Report,
  RemoteExecutorListItem,
  SaveGlobalVariableRequest,
  Settings,
  TriggerBinding,
  UpdateAvailability,
} from "@/lib/types";
import {
  buildLibrary,
  localizeDefinitions,
  pipelineFromBackend,
  type DefinitionIndex,
  type UiFunctionSummary,
  type UiPipeline,
} from "@/lib/adapters";
import { fnSummaryFromBackend } from "@/lib/adapters";
import type { LibraryCategory } from "@/types";

const RUNNING_POLL_MS = 2000;

/**
 * Owns every piece of backend state shared by the shell and the views:
 * workspace lists, settings, node catalog and the running-pipeline poll.
 * Views own their own detail data (databases, chat, docs, metrics).
 */
export function useWorkspace(notify: (text: string, icon?: string) => void) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [pipelines, setPipelines] = useState<UiPipeline[]>([]);
  const [triggers, setTriggers] = useState<TriggerBinding[]>([]);
  const [schedules, setSchedules] = useState<TriggerBinding[]>([]);
  const [reports, setReports] = useState<Report[]>([]);
  const [functions, setFunctions] = useState<UiFunctionSummary[]>([]);
  const [variables, setVariables] = useState<GlobalVariableSummary[]>([]);
  const [definitions, setDefinitions] = useState<ReturnType<typeof localizeDefinitions>>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [executors, setExecutors] = useState<RemoteExecutorListItem[]>([]);
  const [running, setRunning] = useState<Record<string, boolean>>({});
  const [update, setUpdate] = useState<UpdateAvailability | null>(null);

  const reportSeq = useRef(0);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const applySettingsLanguage = useCallback(async (next: Settings | null) => {
    const language = next?.language || "en";
    if (i18n.resolvedLanguage !== language) await i18n.changeLanguage(language);
  }, []);

  const refreshTriggers = useCallback(async () => {
    const [all, crons] = await Promise.all([desktop.listTriggers(), desktop.listSchedules()]);
    if (!mounted.current) return;
    setTriggers(all);
    setSchedules(crons);
  }, []);

  const refreshReports = useCallback(async () => {
    const seq = ++reportSeq.current;
    const next = await desktop.listReports();
    if (!mounted.current || seq !== reportSeq.current) return;
    setReports(next);
  }, []);

  const refreshVariables = useCallback(async () => {
    const next = await desktop.listGlobalVariables();
    if (!mounted.current) return;
    setVariables(next);
  }, []);

  const refreshFunctions = useCallback(async () => {
    const next = await desktop.listFunctions();
    if (!mounted.current) return;
    setFunctions(next.map(fnSummaryFromBackend));
  }, []);

  const refreshExecutors = useCallback(async () => {
    const next = await desktop.listRemoteExecutors();
    if (!mounted.current) return;
    setExecutors(next);
  }, []);

  /**
   * Re-reads persisted settings. Used when a view that renders
   * settings-backed state (identities, defaults) is opened or mutated:
   * services like Discord/Telegram write identities straight into settings
   * server-side, so the workspace snapshot goes stale until re-fetched.
   * Language is deliberately NOT re-applied — an unsaved language edit in
   * the settings draft must survive this refresh.
   */
  const refreshSettings = useCallback(async () => {
    try {
      const next = await desktop.getSettings();
      if (!mounted.current) return;
      setSettings(next);
    } catch {
      /* transient backend hiccup — keep previous settings */
    }
  }, []);

  const refresh = useCallback(async () => {
    try {
      const [pipelineList, allTriggers, crons, reportList, nodes, fnList, variableList, nextSettings, executorList] =
        await Promise.all([
          desktop.listPipelines(),
          desktop.listTriggers(),
          desktop.listSchedules(),
          desktop.listReports(),
          desktop.listNodes(),
          desktop.listFunctions(),
          desktop.listGlobalVariables(),
          desktop.getSettings(),
          desktop.listRemoteExecutors().catch(() => [] as RemoteExecutorListItem[]),
        ]);
      if (!mounted.current) return;
      const localized = localizeDefinitions(nodes);
      setPipelines(pipelineList.map(pipelineFromBackend));
      setTriggers(allTriggers);
      setSchedules(crons);
      setReports(reportList);
      setDefinitions(localized);
      setFunctions(fnList.map(fnSummaryFromBackend));
      setVariables(variableList);
      setSettings(nextSettings);
      setExecutors(executorList);
      setError(null);
      await applySettingsLanguage(nextSettings);
    } catch (err) {
      if (!mounted.current) return;
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      if (mounted.current) setLoading(false);
    }
  }, [applySettingsLanguage]);

  useEffect(() => {
    void refresh();
    void desktop.getUpdateAvailability().then(setUpdate).catch(() => undefined);
  }, [refresh]);

  /* ---------- running-pipeline poll ---------- */

  const pipelineIds = pipelines.map((p) => p.id).join(",");
  useEffect(() => {
    let cancelled = false;
    const poll = async () => {
      const ids = pipelineIds ? pipelineIds.split(",") : [];
      if (ids.length === 0) {
        setRunning({});
        return;
      }
      try {
        const states = await Promise.all(ids.map((id) => desktop.isPipelineRunning(id)));
        if (cancelled) return;
        const next: Record<string, boolean> = {};
        ids.forEach((id, i) => (next[id] = states[i]));
        setRunning(next);
      } catch {
        /* transient backend hiccup — keep previous state */
      }
    };
    void poll();
    const timer = window.setInterval(poll, RUNNING_POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [pipelineIds]);

  /* ---------- derived ---------- */

  const definitionIndex: DefinitionIndex = useMemo(() => {
    const index: DefinitionIndex = {};
    for (const d of definitions) index[d.type] = d;
    return index;
  }, [definitions]);

  const library: LibraryCategory[] = useMemo(
    () => buildLibrary(definitions, []),
    // library labels localize with the language; rebuild on language change
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [definitions, i18n.resolvedLanguage],
  );

  /** Library for the function editor: triggers only ever start pipelines, so
   *  they are hidden from the palette and the canvas add-node picker there. */
  const functionLibrary: LibraryCategory[] = useMemo(
    () => buildLibrary(definitions, [], { excludeTriggers: true }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [definitions, i18n.resolvedLanguage],
  );

  const buttonBindings = useMemo(() => triggers.filter((t) => t.kind === "button"), [triggers]);

  /* ---------- actions ---------- */

  const withNotify = useCallback(
    async <T>(action: () => Promise<T>, failureKey: string): Promise<T | undefined> => {
      try {
        return await action();
      } catch (err) {
        notify(i18n.t(failureKey), "AlertTriangle");
        return undefined;
      }
    },
    [notify],
  );

  const createPipeline = useCallback(
    async (name: string, executorId?: string) => {
      const created = executorId ? await desktop.createPipelineForExecutor(name, executorId) : await desktop.createPipeline(name);
      await refresh();
      return created;
    },
    [refresh],
  );

  const deletePipeline = useCallback(
    async (id: string) => {
      await desktop.deletePipeline(id);
      await refresh();
    },
    [refresh],
  );

  const duplicatePipeline = useCallback(
    async (id: string) => {
      const copy = await desktop.duplicatePipeline(id);
      await refresh();
      return copy;
    },
    [refresh],
  );

  const stopPipeline = useCallback(
    async (id: string) => {
      await withNotify(() => desktop.cancelPipelineExecution(id), "pipelines.stopFailed");
      await refresh();
    },
    [refresh, withNotify],
  );

  const saveSettings = useCallback(
    async (next: Settings) => {
      await desktop.saveSettings(next);
      setSettings(next);
      await applySettingsLanguage(next);
    },
    [applySettingsLanguage],
  );

  const runTrigger = useCallback(
    (id: string) => withNotify(() => desktop.runTrigger(id), "triggers.runFailed"),
    [withNotify],
  );

  return {
    loading,
    error,
    pipelines,
    triggers,
    schedules,
    buttonBindings,
    reports,
    functions,
    variables,
    definitions,
    definitionIndex,
    library,
    functionLibrary,
    settings,
    executors,
    running,
    update,
    notify,
    refresh,
    refreshTriggers,
    refreshReports,
    refreshVariables,
    refreshFunctions,
    refreshExecutors,
    refreshSettings,
    setUpdate,
    createPipeline,
    deletePipeline,
    duplicatePipeline,
    stopPipeline,
    runTrigger,
    saveSettings,
    savePipeline: (p: Pipeline) => desktop.savePipeline(p),
    publishPipeline: (p: Pipeline) => desktop.publishPipeline(p),
    getPipeline: (id: string) => desktop.getPipeline(id),
    listExecutions: (id: string) => desktop.listExecutions(id),
    getExecution: (id: string) => desktop.getExecution(id),
    createFunction: (req: CreateFunctionRequest) => desktop.createFunction(req),
    getFunction: (id: string) => desktop.getFunction(id),
    saveFunction: (item: CustomFunction) => desktop.saveFunction(item),
    publishFunction: (item: CustomFunction) => desktop.publishFunction(item),
    deleteFunction: async (id: string) => {
      await desktop.deleteFunction(id);
      await refreshFunctions();
    },
    createVariable: async (req: SaveGlobalVariableRequest) => {
      await desktop.createGlobalVariable(req);
      await refreshVariables();
    },
    updateVariable: async (req: SaveGlobalVariableRequest) => {
      await desktop.updateGlobalVariable(req);
      await refreshVariables();
    },
    deleteVariable: async (id: string) => {
      await desktop.deleteGlobalVariable(id);
      await refreshVariables();
    },
    deleteReport: async (id: string) => {
      await desktop.deleteReport(id);
      await refreshReports();
    },
    runPipelineDraft: (pipelineId: string, triggerNodeId: string): Promise<Execution> =>
      desktop.runPipelineDraft(pipelineId, triggerNodeId),
    cancelPipelineExecution: (pipelineId: string) => desktop.cancelPipelineExecution(pipelineId),
    resolveNodeDefinition: desktop.resolveNodeDefinition,
  };
}

export type Workspace = ReturnType<typeof useWorkspace>;
