import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { desktop } from "@/lib/bridge";
import type { Execution } from "@/lib/types";
import { formatDateTime, formatDuration } from "@/lib/format";
import type { Workspace } from "@/features/workspace/useWorkspace";
import type { NavApi } from "@/features/workspace/useWorkspaceNav";
import { EmptyState, SearchInput, StatusPill, ViewShell } from "../components/ViewShell";
import { Button } from "../components/ui";
import { Dropdown } from "../components/Dropdown";
import { useCtxMenu } from "../components/ContextMenu";
import { cn } from "../utils/cn";

interface RunRow {
  execution: Execution;
  pipelineName: string;
  triggerLabel: string;
  ms: number;
}

/** Cross-pipeline run history built from ListExecutions. */
export function RunsView({ workspace, nav }: { workspace: Workspace; nav: NavApi }) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [triggerFilter, setTriggerFilter] = useState("all");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [runs, setRuns] = useState<RunRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [nonce, setNonce] = useState(0);
  const ctx = useCtxMenu();

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const rows: RunRow[] = [];
        for (const p of workspace.pipelines) {
          const list = await desktop.listExecutions(p.id);
          if (cancelled) return;
          for (const e of list.slice(0, 50)) {
            const binding = workspace.triggers.find((tr) => tr.id === e.triggerId);
            rows.push({
              execution: e,
              pipelineName: p.name,
              triggerLabel: binding ? t(`triggers.types.${binding.kind}`) : t("runs.manual"),
              ms: Math.max(
                0,
                (e.finishedAt ? Date.parse(e.finishedAt) : Date.now()) - Date.parse(e.startedAt),
              ),
            });
          }
        }
        rows.sort((a, b) => Date.parse(b.execution.startedAt) - Date.parse(a.execution.startedAt));
        if (!cancelled) setRuns(rows);
      } catch {
        /* keep previous */
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [workspace.pipelines, workspace.triggers, t, nonce]);

  const triggerOptions = useMemo(
    () => [
      { value: "all", label: t("runs.allTriggers"), icon: "Zap" },
      { value: "button", label: t("triggers.types.button"), icon: "Grid2x2" },
      { value: "cron", label: t("triggers.types.cron"), icon: "Clock" },
      { value: "chat", label: t("triggers.types.chat"), icon: "MessagesSquare" },
      { value: "webhook", label: t("triggers.types.webhook"), icon: "Radio" },
      { value: "hotkey", label: t("triggers.types.hotkey"), icon: "Command" },
      { value: "file", label: t("triggers.types.file"), icon: "FileText" },
      { value: "twitch", label: t("triggers.types.twitch"), icon: "Radio" },
    ],
    [t],
  );

  const list = runs.filter((r) => {
    const kind = r.execution.triggerId
      ? workspace.triggers.find((tr) => tr.id === r.execution.triggerId)?.kind ?? ""
      : "";
    return (
      (triggerFilter === "all" || kind === triggerFilter) &&
      (r.pipelineName.toLowerCase().includes(q.toLowerCase()) || r.execution.id.includes(q))
    );
  });
  const active = runs.find((r) => r.execution.id === selectedId);
  const slowest = Math.max(...list.map((r) => r.ms), 1);

  const runMenu = (e: React.MouseEvent, r: RunRow) => {
    setSelectedId(r.execution.id);
    ctx(e, [
      { label: t("runs.viewPipeline"), icon: "Cable", onSelect: () => nav.goto("pipelines") },
      { type: "sep" as const },
      {
        label: t("runs.copyRunId"),
        icon: "Copy",
        onSelect: () => navigator.clipboard?.writeText(r.execution.id),
      },
    ]);
  };

  return (
    <ViewShell
      title={t("runs.title")}
      subtitle={t("status.count", { count: runs.length })}
      padded={false}
      actions={
        <Button icon="RefreshCw" onClick={() => setNonce((n) => n + 1)}>
          {t("common.refresh")}
        </Button>
      }
      toolbar={
        <>
          <SearchInput value={q} onChange={setQ} placeholder={t("runs.search")} className="w-[280px]" />
          <Dropdown value={triggerFilter} onChange={setTriggerFilter} className="w-[160px]" options={triggerOptions} />
        </>
      }
    >
      <div className="flex h-full min-h-0">
        <div className="min-w-0 flex-1 overflow-y-auto">
          {loading && runs.length === 0 ? (
            <p className="px-4 py-8 text-center text-[12px] text-fg-faint">{t("common.loading")}</p>
          ) : list.length === 0 ? (
            <EmptyState icon="History" title={t("runs.emptyTitle")} hint={t("runs.emptyDescription")} />
          ) : (
            list.map((r) => (
              <button
                key={r.execution.id}
                onClick={() => setSelectedId(r.execution.id)}
                onContextMenu={(e) => runMenu(e, r)}
                className={cn(
                  "block w-full border-b border-seam/70 px-4 py-2.5 text-left transition",
                  selectedId === r.execution.id ? "bg-ink-850" : "hover:bg-ink-850/60",
                )}
              >
                <div className="flex items-center gap-2">
                  <span className="truncate text-[12.5px] font-medium text-fg">{r.pipelineName}</span>
                  <StatusPill status={r.execution.status} />
                  <span className="ml-auto font-mono text-[10.5px] text-fg-faint">
                    {formatDateTime(r.execution.startedAt)}
                  </span>
                </div>
                <div className="mt-1 flex items-center gap-2">
                  <span className="font-mono text-[10.5px] text-fg-faint">{r.execution.id}</span>
                  <span className="text-[10.5px] text-fg-faint">·</span>
                  <span className="truncate text-[10.5px] text-fg-faint">{r.triggerLabel}</span>
                  {r.execution.executorId && (
                    <span className="inline-flex shrink-0 items-center gap-1 rounded border border-violet-500/30 bg-violet-500/10 px-1 py-px text-[9.5px] font-medium text-violet-300">
                      {t("executors.category")}
                    </span>
                  )}
                  <span className="ml-auto font-mono text-[10.5px] text-fg-subtle">{formatDuration(r.ms)}</span>
                </div>
                <div className="mt-1.5 h-[3px] overflow-hidden rounded-full bg-ink-800">
                  <div
                    className={cn(
                      "h-full rounded-full",
                      r.execution.status === "failed"
                        ? "bg-danger/70"
                        : r.execution.status === "running"
                          ? "bg-ink-100"
                          : "bg-ink-500",
                    )}
                    style={{ width: `${Math.max(3, (r.ms / slowest) * 100)}%` }}
                  />
                </div>
              </button>
            ))
          )}
        </div>

        <aside className="w-[290px] shrink-0 overflow-y-auto border-l border-seam p-3.5">
          {active ? (
            <div className="fade-in space-y-3">
              <div>
                <p className="text-[13px] font-semibold text-fg">{active.pipelineName}</p>
                <p className="mt-0.5 font-mono text-[10.5px] text-fg-faint">{active.execution.id}</p>
              </div>

              <div className="grid grid-cols-2 gap-2">
                {[
                  [t("runs.status"), active.execution.status],
                  [t("runs.trigger"), active.triggerLabel],
                  [t("runs.duration"), formatDuration(active.ms)],
                  [t("runs.nodes"), String(active.execution.nodeRuns?.length ?? 0)],
                ].map(([k, v]) => (
                  <div key={k} className="rounded-lg border border-ink-700/80 bg-ink-850/60 px-2.5 py-2">
                    <p className="text-[10px] tracking-wide text-fg-faint uppercase">{k}</p>
                    <p className="mt-0.5 truncate text-[12px] capitalize text-fg">{v}</p>
                  </div>
                ))}
              </div>

              {active.execution.error && (
                <pre className="max-h-[120px] overflow-auto whitespace-pre-wrap rounded-md border border-danger/20 bg-danger/5 px-2.5 py-2 font-mono text-[10.5px] text-danger-fg">
                  {active.execution.error}
                </pre>
              )}

              <div>
                <p className="mb-1.5 text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">
                  {t("runs.timeline")}
                </p>
                <div className="overflow-hidden rounded-lg border border-ink-700/80">
                  {(active.execution.nodeRuns ?? []).map((step) => (
                    <div key={`${step.nodeId}-${step.startedAt}`} className="flex items-center gap-2 border-b border-seam bg-ink-850/60 px-2.5 py-1.5 last:border-b-0">
                      <DotFor status={step.status} />
                      <span className="truncate font-mono text-[10.5px] text-fg-muted">{step.nodeType || step.nodeId}</span>
                      <span className="ml-auto font-mono text-[10px] text-fg-faint">
                        {formatDuration(Math.max(0, Date.parse(step.finishedAt) - Date.parse(step.startedAt)))}
                      </span>
                    </div>
                  ))}
                </div>
              </div>

              <Button
                icon="Cable"
                variant="solid"
                className="w-full justify-center"
                onClick={() => nav.goto("pipelines")}
              >
                {t("runs.viewPipeline")}
              </Button>
            </div>
          ) : (
            <EmptyState icon="MousePointer2" title={t("runs.selectRun")} />
          )}
        </aside>
      </div>
    </ViewShell>
  );
}

function DotFor({ status }: { status: string }) {
  return (
    <span
      className={cn(
        "h-1.5 w-1.5 shrink-0 rounded-full",
        status === "completed" && "bg-success",
        status === "running" && "bg-ink-100 pulse-ring",
        status === "failed" && "bg-danger",
        status === "skipped" && "bg-ink-600",
        status === "pending" && "bg-warning/70",
        status === "cancelled" && "bg-ink-500",
      )}
    />
  );
}



