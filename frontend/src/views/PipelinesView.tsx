import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { Workspace } from "@/features/workspace/useWorkspace";
import type { NavApi } from "@/features/workspace/useWorkspaceNav";
import { formatDateTime } from "@/lib/format";
import type { UiPipeline } from "@/lib/adapters";
import { desktop } from "@/lib/bridge";
import type { Pipeline, RemoteExecutorListItem } from "@/lib/types";
import { ask } from "@/stores/confirmation";
import { Card, EmptyState, SearchInput, StatusPill, ViewShell } from "../components/ViewShell";
import { Button } from "../components/ui";
import { Icon } from "../components/icons";
import { useCtxMenu } from "../components/ContextMenu";
import { SegmentedControl } from "../components/primitives/SegmentedControl";
import { Modal } from "../components/primitives/Modal";
import { Field, TextInput } from "../components/primitives/Field";
import { Dropdown } from "../components/Dropdown";

type Filter = "all" | "published" | "draft" | "remote";

export default function PipelinesView({ workspace, nav }: { workspace: Workspace; nav: NavApi }) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [newOpen, setNewOpen] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const ctx = useCtxMenu();

  const source = workspace.pipelines;

  const syncToExecutor = async (p: UiPipeline) => {
    try {
      await desktop.syncPipelineToExecutor(p.id);
      workspace.notify(t("executors.synced"), "Check");
    } catch {
      workspace.notify(t("executors.syncFailed"), "AlertTriangle");
    }
  };

  const pipelineMenu = (e: React.MouseEvent, p: UiPipeline) => {
    const items = [
      {
        label: t("pipelines.open"),
        icon: "ArrowUpRight",
        onSelect: () => void nav.openPipeline(p),
      },
      {
        label: t("pipelines.duplicate"),
        icon: "Copy",
        onSelect: () => {
          void workspace.duplicatePipeline(p.id).then((copy) => {
            if (copy) void nav.openPipeline({ ...pipelineFrom(copy) });
          });
        },
      },
      ...(p.executorId
        ? [
            {
              label: t("executors.syncAction"),
              icon: "RefreshCw",
              onSelect: () => void syncToExecutor(p),
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
          (filter === "all" || (filter === "remote" ? Boolean(p.executorId) : p.status === filter)) &&
          (p.name.toLowerCase().includes(q.toLowerCase()) || p.desc.toLowerCase().includes(q.toLowerCase())),
      ),
    [q, filter, source],
  );

  return (
    <ViewShell
      title={t("pipelines.title")}
      subtitle={t("status.count", { count: source.length })}
      actions={<Button icon="Plus" variant="primary" onClick={() => setNewOpen(true)}>{t("pipelines.new")}</Button>}
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
              { value: "remote", label: t("executors.category") },
            ]}
          />
          <span className="ml-auto text-[11.5px] text-fg-faint">{t("pipelines.clickToOpen")}</span>
        </>
      }
    >
      {list.length === 0 ? (
        <EmptyState icon="Cable" title={t("pipelines.emptyTitle")} hint={t("pipelines.emptyDescription")} />
      ) : (
        <div className="overflow-hidden rounded-xl border border-ink-700/80">
          <div className="grid grid-cols-[minmax(0,1fr)_110px_110px_150px_32px] items-center gap-3 border-b border-seam bg-ink-850/70 px-3 py-2 text-[10.5px] font-medium tracking-[0.08em] text-fg-subtle uppercase">
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
                  <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg-subtle transition group-hover:border-ink-500 group-hover:text-fg">
                    <Icon name={isRunning ? "Loader2" : p.icon} className={isRunning ? "h-[15px] w-[15px] animate-spin" : "h-[15px] w-[15px]"} />
                  </span>
                  <span className="min-w-0">
                    <span className="flex items-center gap-1.5">
                      <span className="truncate text-[13px] font-medium text-fg">{p.name}</span>
                      {p.status === "published" && (
                        <span className="font-mono text-[10px] text-fg-faint">{p.version}</span>
                      )}
                    </span>
                    <span className="mt-[1px] flex min-w-0 items-center gap-1.5">
                      {p.executorName && (
                        <span className="inline-flex max-w-full shrink-0 items-center gap-1 rounded border border-violet-500/30 bg-violet-500/10 px-1 py-px text-[9.5px] font-medium text-violet-300">
                          <Icon name="Server" className="h-2.5 w-2.5 shrink-0" />
                          <span className="truncate">{p.executorName}</span>
                        </span>
                      )}
                      <span className="truncate text-[11.5px] text-fg-faint">{p.desc}</span>
                    </span>
                  </span>
                </span>
                <span>
                  {isRunning ? <StatusPill status="running" /> : <StatusPill status={p.status} />}
                </span>
                <span className="text-right font-mono text-[11.5px] text-fg-subtle">
                  {p.triggers > 0 ? t("pipelines.triggerCount", { count: p.triggers }) : "–"}
                </span>
                <span className="truncate text-[11.5px] text-fg-subtle">{p.updated}</span>
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

      <p className="mt-3 flex items-center gap-1.5 px-1 text-[11.5px] text-fg-faint">
        <Icon name="Info" className="h-3.5 w-3.5 shrink-0" />
        {t("pipelines.boardNote")}
      </p>

      <Card onClick={() => setNewOpen(true)} hoverable className="mt-4 flex items-center gap-3 p-3">
        <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg-subtle">
          <Icon name="Plus" className="h-4 w-4" />
        </span>
        <span>
          <span className="block text-[12.5px] font-medium text-fg">{t("pipelines.blankTitle")}</span>
          <span className="block text-[11px] text-fg-faint">{t("pipelines.blankHint")}</span>
        </span>
      </Card>

      <CreatePipelineDialog
        open={newOpen}
        executors={workspace.executors}
        onClose={() => setNewOpen(false)}
        onCreate={async (name, executorId) => {
          const created: Pipeline | undefined = await workspace.createPipeline(name, executorId || undefined);
          if (!created) return;
          setNewOpen(false);
          await nav.openPipeline({ ...pipelineFrom(created) });
        }}
      />
    </ViewShell>
  );
}

function cnStop(running: boolean) {
  return running
    ? "grid h-6 w-6 place-items-center rounded text-danger-fg transition hover:bg-danger/15"
    : "grid h-6 w-6 place-items-center rounded text-fg-faint transition group-hover:text-fg";
}

/** Adapts a freshly returned Pipeline into the summary shape for the editor nav. */
function pipelineFrom(created: Pipeline): UiPipeline {
  return {
    id: created.id,
    name: created.name,
    desc: created.description,
    icon: created.icon || "Cable",
    status: created.status === "active" ? "published" : "draft",
    version: created.publishedRevision > 0 ? `v${created.publishedRevision}` : "",
    triggers: 0,
    updated: formatDateTime(created.updatedAt),
    executorId: created.executorId || undefined,
  };
}

/** Dialog choosing a name and the run target: this machine or one remote executor. */
export function CreatePipelineDialog({
  open,
  executors,
  onClose,
  onCreate,
}: {
  open: boolean;
  executors: RemoteExecutorListItem[];
  onClose: () => void;
  onCreate: (name: string, executorId?: string) => Promise<void>;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [target, setTarget] = useState("");

  if (!open) return null;

  const close = () => {
    onClose();
    setName("");
    setTarget("");
  };

  const valid = name.trim().length > 0;

  return (
    <Modal title={t("pipelines.createTitle")} onClose={close}>
      <div className="space-y-3">
        <Field label={t("pipelines.nameLabel")} required>
          <TextInput value={name} onChange={setName} autoFocus placeholder={t("pipelines.namePlaceholder")} />
        </Field>
        <Field label={t("executors.targetLabel")} hint={t("executors.targetHint")}>
          <Dropdown
            value={target}
            onChange={setTarget}
            options={[
              { value: "", label: t("executors.localTarget"), icon: "Laptop" },
              ...executors.map((e) => ({
                value: e.id,
                label: e.name,
                icon: "Server",
                hint: e.status.online ? undefined : t("executors.offlineHint"),
              })),
            ]}
          />
        </Field>
      </div>
      <div className="ml-auto flex items-center gap-2">
        <Button variant="ghost" onClick={close}>
          {t("common.cancel")}
        </Button>
        <Button
          variant="primary"
          icon="Plus"
          disabled={!valid}
          onClick={() => {
            void onCreate(name.trim(), target || undefined).then(() => {
              setName("");
              setTarget("");
            });
          }}
        >
          {t("pipelines.new")}
        </Button>
      </div>
    </Modal>
  );
}
