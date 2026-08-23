import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { Workspace } from "@/features/workspace/useWorkspace";
import type { NavApi } from "@/features/workspace/useWorkspaceNav";
import { formatDateTime } from "@/lib/format";
import type { UiPipeline } from "@/lib/adapters";
import { ask } from "@/stores/confirmation";
import { Card, EmptyState, SearchInput, StatusPill, ViewShell } from "../components/ViewShell";
import { Button } from "../components/ui";
import { Icon } from "../components/icons";
import { useCtxMenu } from "../components/ContextMenu";
import { SegmentedControl } from "../components/primitives/SegmentedControl";

type Filter = "all" | "published" | "draft" | "legacy";

export default function PipelinesView({ workspace, nav }: { workspace: Workspace; nav: NavApi }) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [newName, setNewName] = useState("");
  const [busyId, setBusyId] = useState<string | null>(null);
  const ctx = useCtxMenu();

  const source = workspace.pipelines;

  const pipelineMenu = (e: React.MouseEvent, p: UiPipeline) => {
    const items = [
      {
        label: t("pipelines.open"),
        icon: "ArrowUpRight",
        onSelect: () => void nav.openPipeline(p),
      },
      ...(p.status !== "legacy"
        ? [
            {
              label: t("pipelines.duplicate"),
              icon: "Copy",
              onSelect: () => {
                void workspace.duplicatePipeline(p.id).then((copy) => {
                  if (copy) void nav.openPipeline({ ...pipelineFrom(copy) });
                });
              },
            },
          ]
        : []),
      { type: "sep" as const },
      {
        label: t("common.delete"),
        icon: "Trash2",
        danger: true,
        onSelect: async () => {
          const ok = await ask({
            title: t("pipelines.deleteTitle"),
            description: t("pipelines.deleteDescription", { name: p.name }),
            confirmLabel: t("pipelines.deleteConfirm"),
            danger: true,
          });
          if (!ok) return;
          setBusyId(p.id);
          await workspace.deletePipeline(p.id);
          setBusyId(null);
        },
      },
    ];
    ctx(e, items);
  };

  const list = useMemo(
    () =>
      source.filter(
        (p) =>
          (filter === "all" || p.status === filter) &&
          (p.name.toLowerCase().includes(q.toLowerCase()) || p.desc.toLowerCase().includes(q.toLowerCase())),
      ),
    [q, filter, source],
  );

  const create = async () => {
    const name = newName.trim() || t("pipelines.namePlaceholder");
    try {
      const created = await workspace.createPipeline(name);
      setNewName("");
      if (created) await nav.openPipeline(pipelineFrom(created));
    } catch {
      workspace.notify(t("pipelines.createFailed"), "AlertTriangle");
    }
  };

  return (
    <ViewShell
      title={t("pipelines.title")}
      subtitle={t("status.count", { count: source.length })}
      actions={
        <Button icon="Plus" variant="primary" onClick={() => void create()}>
          {t("pipelines.new")}
        </Button>
      }
      toolbar={
        <>
          <SearchInput value={q} onChange={setQ} placeholder={t("pipelines.search")} className="w-[240px]" />
          <SegmentedControl
            value={filter}
            onChange={(v) => setFilter(v)}
            segments={[
              { value: "all", label: t("common.all") },
              { value: "published", label: t("pipelines.publishedTab") },
              { value: "draft", label: t("functions.draft") },
              { value: "legacy", label: t("pipelines.legacy") },
            ]}
          />
          <span className="ml-auto text-[11.5px] text-ink-500">{t("pipelines.clickToOpen")}</span>
        </>
      }
    >
      {list.length === 0 ? (
        <EmptyState icon="Cable" title={t("pipelines.emptyTitle")} hint={t("pipelines.emptyDescription")} />
      ) : (
        <div className="overflow-hidden rounded-xl border border-ink-700/80">
          <div className="grid grid-cols-[minmax(0,1fr)_110px_110px_150px_32px] items-center gap-3 border-b border-seam bg-ink-850/70 px-3 py-2 text-[10.5px] font-medium tracking-[0.08em] text-ink-400 uppercase">
            <span>{t("pipelines.pipeline")}</span>
            <span>{t("pipelines.status")}</span>
            <span className="text-right">{t("pipelines.triggersHeader")}</span>
            <span>{t("pipelines.updated")}</span>
            <span />
          </div>
          {list.map((p) => {
            const isRunning = Boolean(workspace.running[p.id]);
            return (
              <button
                key={p.id}
                onClick={() => void nav.openPipeline(p)}
                onContextMenu={(e) => pipelineMenu(e, p)}
                disabled={busyId === p.id}
                className="group grid w-full grid-cols-[minmax(0,1fr)_110px_110px_150px_32px] items-center gap-3 border-b border-seam/70 px-3 py-2.5 text-left transition last:border-b-0 hover:bg-ink-850"
              >
                <span className="flex min-w-0 items-center gap-2.5">
                  <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-ink-300 transition group-hover:border-ink-500 group-hover:text-ink-50">
                    <Icon name={isRunning ? "Loader2" : p.icon} className={isRunning ? "h-[15px] w-[15px] animate-spin" : "h-[15px] w-[15px]"} />
                  </span>
                  <span className="min-w-0">
                    <span className="flex items-center gap-1.5">
                      <span className="truncate text-[13px] font-medium text-ink-50">{p.name}</span>
                      {p.status === "published" && (
                        <span className="font-mono text-[10px] text-ink-500">{p.version}</span>
                      )}
                      {p.migrationIssue && (
                        <Icon name="AlertTriangle" className="h-3 w-3 shrink-0 text-amber-300" />
                      )}
                    </span>
                    <span className="mt-[1px] block truncate text-[11.5px] text-ink-500">{p.desc}</span>
                  </span>
                </span>
                <span>
                  {isRunning ? <StatusPill status="running" /> : <StatusPill status={p.status} />}
                </span>
                <span className="text-right font-mono text-[11.5px] text-ink-300">
                  {p.triggers > 0 ? t("pipelines.triggerCount", { count: p.triggers }) : "–"}
                </span>
                <span className="truncate text-[11.5px] text-ink-400">{p.updated}</span>
                <span
                  role="button"
                  tabIndex={-1}
                  onClick={(e) => {
                    e.stopPropagation();
                    if (isRunning) void workspace.stopPipeline(p.id);
                  }}
                  className={cnStop(isRunning)}
                >
                  <Icon name={isRunning ? "Square" : "ChevronRight"} className="h-4 w-4" />
                </span>
              </button>
            );
          })}
        </div>
      )}

      <p className="mt-3 flex items-center gap-1.5 px-1 text-[11.5px] text-ink-500">
        <Icon name="Info" className="h-3.5 w-3.5 shrink-0" />
        {t("pipelines.boardNote")}
      </p>

      <Card onClick={() => void create()} hoverable className="mt-4 flex items-center gap-3 p-3">
        <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-ink-300">
          <Icon name="Plus" className="h-4 w-4" />
        </span>
        <span>
          <span className="block text-[12.5px] font-medium text-ink-100">{t("pipelines.blankTitle")}</span>
          <span className="block text-[11px] text-ink-500">{t("pipelines.blankHint")}</span>
        </span>
      </Card>
    </ViewShell>
  );
}

function cnStop(running: boolean) {
  return running
    ? "grid h-6 w-6 place-items-center rounded text-rose-300 transition hover:bg-rose-500/15"
    : "grid h-6 w-6 place-items-center rounded text-ink-600 transition group-hover:text-ink-100";
}

/** Adapts a freshly returned Pipeline into the summary shape for the editor nav. */
function pipelineFrom(created: import("@/lib/types").Pipeline): UiPipeline {
  return {
    id: created.id,
    name: created.name,
    desc: created.description,
    icon: created.icon || "Cable",
    status: created.status === "active" ? "published" : created.status === "legacy" ? "legacy" : "draft",
    version: created.publishedRevision > 0 ? `v${created.publishedRevision}` : "",
    triggers: 0,
    updated: formatDateTime(created.updatedAt),
  };
}

