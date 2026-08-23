import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import { desktop } from "@/lib/bridge";
import type { MetricsOverview } from "@/lib/types";
import { formatCompact, formatDuration, formatNumber, formatPercent, formatUsd } from "@/lib/format";
import { usePersistedChoice } from "@/lib/prefs";
import { Tooltip } from "../components/Tooltip";
import { Card, EmptyState, ViewShell } from "../components/ViewShell";
import { Icon } from "../components/icons";
import { cn } from "../utils/cn";

const RANGES = [
  { id: "today", ms: 86_400_000 },
  { id: "24h", ms: 86_400_000 },
  { id: "7d", ms: 7 * 86_400_000 },
  { id: "30d", ms: 30 * 86_400_000 },
  { id: "90d", ms: 90 * 86_400_000 },
] as const;

type RangeId = (typeof RANGES)[number]["id"];

export function MetricsView() {
  const { t } = useTranslation();
  const [range, setRange] = usePersistedChoice<RangeId>(
    "neuropipe.metrics.range.v2",
    RANGES.map((r) => r.id),
    "30d",
  );
  const [overview, setOverview] = useState<MetricsOverview | null>(null);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    try {
      const preset = RANGES.find((r) => r.id === range)!;
      const to = new Date();
      const from = new Date(Date.now() - preset.ms);
      setOverview(await desktop.getMetricsOverview({ from: from.toISOString(), to: to.toISOString() }));
    } catch {
      setOverview(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    const off = Events.On("metrics.updated", () => void load());
    return () => off();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [range]);

  const maxRuns = useMemo(
    () => Math.max(1, ...(overview?.runSeries ?? []).map((p) => p.completed + p.failed + p.skipped + p.cancelled)),
    [overview],
  );

  if (loading && !overview) {
    return (
      <ViewShell title={t("metrics.title")} subtitle={t("metrics.description")}>
        <EmptyState icon="Activity" title={t("common.loading")} />
      </ViewShell>
    );
  }

  if (!overview) {
    return (
      <ViewShell title={t("metrics.title")} subtitle={t("metrics.description")}>
        <EmptyState icon="AlertTriangle" title={t("metricsUI.loadFailed")} />
      </ViewShell>
    );
  }

  const kpis = [
    {
      label: t("metricsUI.totalRuns"),
      value: formatNumber(overview.runs.value),
      delta: delta(overview.runs),
      ok: overview.runs.value >= overview.runs.previousValue,
      available: overview.runs.available,
    },
    {
      label: t("metricsUI.successRate"),
      value: formatPercent(overview.successRate.value),
      delta: delta(overview.successRate, true),
      ok: overview.successRate.value >= overview.successRate.previousValue,
      available: overview.successRate.available,
    },
    {
      label: t("metricsUI.p95Duration"),
      value: formatDuration(overview.p95DurationMs.value),
      delta: delta(overview.p95DurationMs),
      ok: overview.p95DurationMs.value <= overview.p95DurationMs.previousValue,
      available: overview.p95DurationMs.available,
      invert: true,
    },
    {
      label:
        overview.unpricedCalls > 0
          ? t("metricsUI.unpriced")
          : overview.localCalls > 0
            ? t("metricsUI.local")
            : t("metricsUI.estimatedCost"),
      value: overview.unpricedCalls > 0 || overview.localCalls > 0 ? "–" : formatUsd(overview.estimatedCostUsd.value),
      delta: delta(overview.estimatedCostUsd),
      ok: overview.estimatedCostUsd.value <= overview.estimatedCostUsd.previousValue,
      available: overview.estimatedCostUsd.available,
    },
  ];

  const top = [...overview.pipelines]
    .reduce((acc, row) => {
      const hit = acc.find((x) => x.pipelineId === row.pipelineId);
      if (hit) {
        hit.runs += row.completed + row.failed;
        hit.failed += row.failed;
      } else {
        acc.push({
          pipelineId: row.pipelineId,
          name: row.name,
          runs: row.completed + row.failed,
          failed: row.failed,
        });
      }
      return acc;
    }, [] as { pipelineId: string; name: string; runs: number; failed: number }[])
    .sort((a, b) => b.runs - a.runs)
    .slice(0, 8);

  const topMax = Math.max(1, ...top.map((x) => x.runs));

  return (
    <ViewShell
      title={t("metrics.title")}
      subtitle={t("metrics.description")}
      toolbar={
        <div className="flex items-center gap-0.5 rounded-md border border-ink-700 bg-ink-850 p-0.5">
          {RANGES.map((r) => (
            <button
              key={r.id}
              onClick={() => setRange(r.id)}
              className={cn(
                "rounded px-2 py-[3px] text-[11.5px] transition",
                range === r.id ? "bg-ink-700 text-ink-50" : "text-ink-400 hover:text-ink-100",
              )}
            >
              {t(`metricsUI.range_${r.id}`)}
            </button>
          ))}
        </div>
      }
    >
      <div className="grid grid-cols-4 gap-3">
        {kpis.map((c) => (
          <Card key={c.label} className="p-3.5">
            <p className="text-[10.5px] tracking-wide text-ink-500 uppercase">{c.label}</p>
            <p className="mt-1 text-[20px] font-semibold tracking-tight text-ink-50">{c.value}</p>
            {c.available && (
              <p
                className={cn(
                  "mt-1 flex items-center gap-1 text-[11px]",
                  c.ok ? "text-emerald-300/80" : "text-amber-300/80",
                )}
              >
                <Icon name={c.delta >= 0 ? "TrendingUp" : "History"} className="h-3 w-3" />
                {formatNumber(Math.abs(Math.round(c.delta * 10) / 10))}% {t("metricsUI.previous")}
              </p>
            )}
          </Card>
        ))}
      </div>

      {/* run health */}
      <Card className="mt-3 p-4">
        <div className="mb-3 flex items-center gap-3">
          <h3 className="text-[12.5px] font-medium text-ink-100">{t("metricsUI.runHealth")}</h3>
          <span className="ml-auto flex items-center gap-3 text-[10.5px] text-ink-500">
            <Legend color="#34d399" label={t("metricsUI.completed")} />
            <Legend color="#fb7185" label={t("metricsUI.failed")} />
            <Legend color="#a1a1aa" label={t("metricsUI.skipped")} />
          </span>
        </div>
        {overview.runSeries.length === 0 ? (
          <p className="py-6 text-center text-[12px] text-ink-500">{t("metricsUI.healthEmpty")}</p>
        ) : (
          <>
            <div className="flex h-[132px] items-end gap-[3px]">
              {overview.runSeries.map((point, i) => {
                const total = point.completed + point.failed + point.skipped + point.cancelled;
                return (
                  <Tooltip
                    key={i}
                    side="top"
                    delay={120}
                    className="min-w-0 flex-1 items-end"
                    content={`${t("metricsUI.totalRuns")}: ${total} · ${t("metricsUI.completed")}: ${point.completed} · ${t("metricsUI.failed")}: ${point.failed}`}
                  >
                    <div
                      role="img"
                      aria-label={`${t("metricsUI.totalRuns")}: ${total}`}
                      className="relative flex w-full"
                      style={{ height: `${Math.max(3, (total / maxRuns) * 120)}px` }}
                    >
                      {point.completed > 0 && (
                        <div className="w-full rounded-t-[2px] bg-emerald-400/70" style={{ flexGrow: point.completed }} />
                      )}
                      {point.failed > 0 && (
                        <div className="w-full bg-rose-400/80" style={{ height: `${Math.max(2, (point.failed / total) * ((total / maxRuns) * 120))}px` }} />
                      )}
                    </div>
                  </Tooltip>
                );
              })}
            </div>
            <div className="mt-1.5 flex justify-between font-mono text-[10px] text-ink-600">
              <span>{timeLabel(overview.runSeries[0]?.at)}</span>
              <span>{timeLabel(overview.runSeries[Math.floor(overview.runSeries.length / 2)]?.at)}</span>
              <span>{timeLabel(overview.runSeries[overview.runSeries.length - 1]?.at)}</span>
            </div>
          </>
        )}
      </Card>

      {/* pipeline health */}
      <Card className="mt-3 overflow-hidden">
        <div className="border-b border-seam px-3.5 py-2.5">
          <h3 className="text-[12.5px] font-medium text-ink-100">{t("metricsUI.pipelineHealth")}</h3>
        </div>
        {top.length === 0 && (
          <p className="px-3.5 py-4 text-[12px] text-ink-500">{t("metricsUI.healthEmpty")}</p>
        )}
        {top.map((row, i) => (
          <div key={row.pipelineId} className={cn("flex items-center gap-3 px-3.5 py-2", i > 0 && "border-t border-seam/70")}>
            <span className="w-[170px] shrink-0 truncate text-[12px] text-ink-100">{row.name}</span>
            <div className="h-[5px] flex-1 overflow-hidden rounded-full bg-ink-800">
              <div
                className={cn("h-full rounded-full", row.failed > 0 ? "bg-gradient-to-r from-emerald-400/70 to-rose-400/70" : "bg-emerald-400/60")}
                style={{ width: `${(row.runs / topMax) * 100}%` }}
              />
            </div>
            <span className="w-14 shrink-0 text-right font-mono text-[11px] text-ink-300">{formatCompact(row.runs)}</span>
            <span className="w-16 shrink-0 text-right font-mono text-[11px] text-rose-300/80">
              {row.failed > 0 ? `${formatCompact(row.failed)} ${t("metricsUI.failed")}` : ""}
            </span>
          </div>
        ))}
      </Card>

      {/* failures + slowest nodes */}
      <div className="mt-3 grid grid-cols-2 gap-3">
        <BreakdownTable title={t("metrics.failuresTitle")} rows={overview.failures} emptyText={t("editorActions.noNodeResults")} />
        <BreakdownTable
          title={t("metrics.slowNodesTitle")}
          rows={overview.slowNodes.map((n) => ({ ...n, secondaryFormatted: formatDuration(n.secondary ?? 0) }))}
          emptyText={t("editorActions.noNodeResults")}
          duration
        />
      </div>

      <p className="mt-3 px-1 text-[11px] leading-relaxed text-ink-600">{t("metrics.privacyNote")}</p>
    </ViewShell>
  );
}

function Legend({ color, label }: { color: string; label: string }) {
  return (
    <span className="flex items-center gap-1">
      <span className="h-1.5 w-1.5 rounded-full" style={{ background: color }} />
      {label}
    </span>
  );
}

function delta(kpi: { value: number; previousValue: number; available: boolean }, percent = false): number {
  if (!kpi.available || kpi.previousValue === 0) return 0;
  // percent-type KPIs (success rate) are already on a 0–100 scale, so their
  // delta is plain percentage points; everything else is relative change.
  if (percent) return kpi.value - kpi.previousValue;
  return ((kpi.value - kpi.previousValue) / kpi.previousValue) * 100;
}

function timeLabel(at?: string): string {
  if (!at) return "";
  const d = new Date(at);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

function BreakdownTable({
  title,
  rows,
  emptyText,
  duration,
}: {
  title: string;
  rows: { id: string; label: string; value: number; secondary?: number; secondaryFormatted?: string }[];
  emptyText: string;
  duration?: boolean;
}) {
  return (
    <Card className="overflow-hidden">
      <div className="border-b border-seam px-3.5 py-2.5">
        <h3 className="text-[12.5px] font-medium text-ink-100">{title}</h3>
      </div>
      {rows.length === 0 && <p className="px-3.5 py-3 text-[12px] text-ink-500">{emptyText}</p>}
      {rows.slice(0, 6).map((row, i) => (
        <div key={row.id} className={cn("flex items-center gap-3 px-3.5 py-2", i > 0 && "border-t border-seam/70")}>
          <span className="min-w-0 flex-1 truncate text-[12px] text-ink-200">{row.label}</span>
          <span className="font-mono text-[11px] text-ink-300">{formatCompact(row.value)}</span>
          {duration !== undefined && (
            <span className="font-mono text-[10.5px] text-ink-500">
              {duration ? (row.secondaryFormatted ?? "") : ""}
            </span>
          )}
        </div>
      ))}
    </Card>
  );
}

/* ---------------- Documentation ---------------- */

export { DocsView } from "./DocsView";


