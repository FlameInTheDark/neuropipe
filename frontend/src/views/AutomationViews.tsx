import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { desktop as bridge } from "@/lib/bridge";
import type { TriggerBinding } from "@/lib/types";
import type { Workspace } from "@/features/workspace/useWorkspace";
import type { NavApi } from "@/features/workspace/useWorkspaceNav";
import type { UiFunctionSummary } from "@/lib/adapters";
import { formatDateTime } from "@/lib/format";
import { Card, EmptyState, SearchInput, StatusPill, ViewShell } from "../components/ViewShell";
import { Tooltip } from "../components/Tooltip";
import { Button, Toggle } from "../components/ui";
import { Icon } from "../components/icons";
import { SegmentedControl } from "../components/primitives/SegmentedControl";
import { Modal, ModalActions } from "../components/primitives/Modal";
import { Field, TextInput, TextArea } from "../components/primitives/Field";
import { cn } from "../utils/cn";
import { useCtxMenu } from "../components/ContextMenu";
import { ask } from "@/stores/confirmation";

/* ---------------- Triggers ---------------- */

const KIND_ICON: Record<string, string> = {
  button: "Grid2x2",
  cron: "Clock",
  file: "FileText",
  hotkey: "Command",
  webhook: "Radio",
  chat: "MessagesSquare",
  twitch: "Radio",
  kvsubscribe: "Database",
  discord: "Hash",
  telegram: "Send",
};

export function TriggersView({ workspace, nav }: { workspace: Workspace; nav: NavApi }) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [busyId, setBusyId] = useState<string | null>(null);

  const list = workspace.triggers.filter((tr) =>
    `${tr.label} ${tr.nodeType ?? ""} ${tr.kind}`.toLowerCase().includes(q.toLowerCase()),
  );

  const pipelineName = (binding: TriggerBinding) =>
    workspace.pipelines.find((p) => p.id === binding.pipelineId)?.name ?? binding.pipelineId;

  const run = async (binding: TriggerBinding) => {
    setBusyId(binding.id);
    await workspace.runTrigger(binding.id);
    setBusyId(null);
  };

  const stop = async (binding: TriggerBinding) => {
    setBusyId(binding.id);
    await workspace.stopPipeline(binding.pipelineId);
    setBusyId(null);
  };

  return (
    <ViewShell
      title={t("triggers.title")}
      subtitle={t("status.count", { count: workspace.triggers.length })}
      toolbar={<SearchInput value={q} onChange={setQ} placeholder={t("triggers.search")} className="w-[260px]" />}
    >
      {list.length === 0 ? (
        <EmptyState icon="CircleDot" title={t("triggers.emptyTitle")} hint={t("triggers.emptyDescription")} />
      ) : (
        <div className="overflow-hidden rounded-xl border border-ink-700/80">
          <div className="grid grid-cols-[minmax(0,1fr)_130px_110px_130px_120px] gap-3 border-b border-seam bg-ink-850/70 px-3 py-2 text-[10.5px] font-medium tracking-[0.08em] text-fg-subtle uppercase">
            <span>{t("nav.triggers")}</span>
            <span>{t("pipelines.pipeline")}</span>
            <span>{t("pipelines.status")}</span>
            <span>{t("triggers.lastRun")}</span>
            <span className="text-right">{t("common.active")}</span>
          </div>
          {list.map((tr) => {
            const running = Boolean(workspace.running[tr.pipelineId]);
            return (
              <div
                key={tr.id}
                className="grid grid-cols-[minmax(0,1fr)_130px_110px_130px_120px] items-center gap-3 border-b border-seam/70 px-3 py-2.5 transition last:border-b-0 hover:bg-ink-850"
              >
                <span className="flex min-w-0 items-center gap-2.5">
                  <Tooltip content={t("board.openPipelines")} side="top">
                    <button
                      onClick={() => nav.goto("pipelines")}
                      aria-label={t("board.openPipelines")}
                      className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg-subtle transition hover:border-ink-500 hover:text-fg"
                    >
                      <Icon name={KIND_ICON[tr.kind] ?? "CircleDot"} className="h-4 w-4" />
                    </button>
                  </Tooltip>
                  <span className="min-w-0">
                    <span className="block truncate text-[12.5px] font-medium text-fg">{tr.label}</span>
                    <span className="block text-[11px] text-fg-faint">{t(`triggers.types.${tr.kind}`)}</span>
                  </span>
                </span>
                <span className="truncate text-[12px] text-fg-subtle">{pipelineName(tr)}</span>
                <StatusPill status={tr.enabled ? "connected" : "idle"} />
                <span className="font-mono text-[11px] text-fg-faint">{formatDateTime(tr.lastRunAt)}</span>
                <span className="flex items-center justify-end gap-1.5">
                  {running ? (
                    <Button icon="Square" variant="solid" onClick={() => void stop(tr)} disabled={busyId === tr.id}>
                      {t("triggers.stop")}
                    </Button>
                  ) : (
                    tr.kind !== "chat" &&
                    tr.kind !== "twitch" &&
                    tr.kind !== "kvsubscribe" &&
                    tr.kind !== "discord" &&
                    tr.kind !== "telegram" && (
                      <Button
                        icon={busyId === tr.id ? "Loader2" : "Play"}
                        variant="solid"
                        spin={busyId === tr.id}
                        onClick={() => void run(tr)}
                      >
                        {t("triggers.run")}
                      </Button>
                    )
                  )}
                </span>
              </div>
            );
          })}
        </div>
      )}
      <p className="mt-3 flex items-center gap-1.5 px-1 text-[11.5px] text-fg-faint">
        <Icon name="Info" className="h-3.5 w-3.5 shrink-0" />
        {t("triggers.boardNote")}
      </p>
    </ViewShell>
  );
}

/* ---------------- Schedules ---------------- */

export function SchedulesView({ workspace }: { workspace: Workspace }) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [busyId, setBusyId] = useState<string | null>(null);

  const list = workspace.schedules.filter(
    (s) => `${s.label} ${s.cron ?? ""}`.toLowerCase().includes(q.toLowerCase()),
  );

  const pipelineName = (binding: TriggerBinding) =>
    workspace.pipelines.find((p) => p.id === binding.pipelineId)?.name ?? binding.pipelineId;

  const trust = async (s: TriggerBinding) => {
    setBusyId(s.id);
    try {
      await bridge.trustRevision(s.pipelineId, s.revision);
      await workspace.refreshTriggers();
    } finally {
      setBusyId(null);
    }
  };

  const toggle = async (s: TriggerBinding, enabled: boolean) => {
    setBusyId(s.id);
    try {
      await bridge.setScheduleEnabled(s.id, enabled);
      await workspace.refreshTriggers();
    } finally {
      setBusyId(null);
    }
  };

  return (
    <ViewShell
      title={t("schedules.title")}
      subtitle={t("status.count", { count: workspace.schedules.length })}
      toolbar={<SearchInput value={q} onChange={setQ} placeholder={t("schedules.search")} className="w-[260px]" />}
    >
      {list.length === 0 ? (
        <EmptyState icon="Clock" title={t("schedules.emptyTitle")} hint={t("schedules.emptyDescription")} />
      ) : (
        <div className="overflow-hidden rounded-xl border border-ink-700/80">
          {list.map((s) => (
            <div
              key={s.id}
              className="flex items-center gap-3 border-b border-seam/70 px-3 py-2.5 transition last:border-b-0 hover:bg-ink-850"
            >
              <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg-subtle">
                <Icon name="Clock" className="h-4 w-4" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[13px] font-medium text-fg">{s.label}</span>
                <span className="block truncate text-[11.5px] text-fg-faint">{pipelineName(s)}</span>
              </span>
              <code className="rounded-md border border-ink-700/70 bg-ink-900/50 px-2.5 py-1.5 font-mono text-[11.5px] text-fg-muted">
                {s.cron}
              </code>
              <span className="w-[170px] shrink-0 text-[11px] text-fg-faint">
                {t("schedules.nextLast", {
                  next: formatDateTime(s.nextRunAt),
                  last: formatDateTime(s.lastRunAt),
                })}
              </span>
              {!s.trusted ? (
                <Button icon="ShieldCheck" variant="solid" onClick={() => void trust(s)} disabled={busyId === s.id}>
                  {t("schedules.trust")}
                </Button>
              ) : (
                <span className="flex items-center gap-1.5 rounded bg-success/10 px-2 py-1 text-[10.5px] font-medium text-success-fg">
                  <Icon name="ShieldCheck" className="h-3 w-3" />
                  {t("schedules.trusted", { version: s.revision })}
                </span>
              )}
              <Toggle on={s.enabled} disabled={!s.trusted} onChange={(v) => void toggle(s, v)} />
            </div>
          ))}
        </div>
      )}
    </ViewShell>
  );
}

/* ---------------- Functions ---------------- */

const FN_KIND_META: Record<UiFunctionSummary["kind"], { labelKey: string; icon: string; tone: string }> = {
  pure: { labelKey: "functions.pure", icon: "Sparkles", tone: "bg-info/10 text-info-fg" },
  impure: { labelKey: "functions.impure", icon: "Zap", tone: "bg-warning/10 text-warning-fg" },
  tool: { labelKey: "functions.tool", icon: "Bot", tone: "bg-success/10 text-success-fg" },
};

function FunctionCreateDialog({
  onCancel,
  onCreate,
}: {
  onCancel: () => void;
  onCreate: (req: { name: string; description: string; kind: "function" | "tool"; mode: "pure" | "impure"; template: string }) => void;
}) {
  const { t } = useTranslation();
  const [template, setTemplate] = useState<"workflow" | "pure" | "tool">("workflow");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  const templates: { id: typeof template; kind: "function" | "tool"; mode: "pure" | "impure" }[] = [
    { id: "workflow", kind: "function", mode: "impure" },
    { id: "pure", kind: "function", mode: "pure" },
    { id: "tool", kind: "tool", mode: "impure" },
  ];

  return (
    <Modal
      title={t("functions.createTitle")}
      icon="Braces"
      onClose={onCancel}
      footer={
        <ModalActions
          onCancel={onCancel}
          onConfirm={() => {
            const tpl = templates.find((x) => x.id === template)!;
            onCreate({ name: name.trim(), description: description.trim(), kind: tpl.kind, mode: tpl.mode, template });
          }}
          confirmLabel={t("functions.create")}
          disabled={!name.trim() || !description.trim()}
        />
      }
    >
      <div className="space-y-3">
        <div className="grid grid-cols-3 gap-2">
          {templates.map((tpl) => (
            <button
              key={tpl.id}
              onClick={() => setTemplate(tpl.id)}
              className={cn(
                "rounded-lg border p-2.5 text-left transition",
                template === tpl.id
                  ? "border-ink-400 bg-ink-750"
                  : "border-ink-700 bg-ink-850 hover:border-ink-500",
              )}
            >
              <span className="block text-[12px] font-semibold text-fg">{t(`functions.types.${tpl.id}.title`)}</span>
              <span className="mt-1 block line-clamp-3 text-[10.5px] leading-snug text-fg-subtle">
                {t(`functions.types.${tpl.id}.description`)}
              </span>
            </button>
          ))}
        </div>
        <Field label={t("functions.nameLabel")} required>
          <TextInput autoFocus value={name} onChange={setName} placeholder={t("functions.namePlaceholder")} />
        </Field>
        <Field label={t("functions.shortDescription")} required>
          <TextArea value={description} onChange={setDescription} placeholder={t("functions.descriptionPlaceholder")} />
        </Field>
      </div>
    </Modal>
  );
}

export function FunctionsView({ workspace, nav }: { workspace: Workspace; nav: NavApi }) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [kind, setKind] = useState<"all" | UiFunctionSummary["kind"]>("all");
  const [createOpen, setCreateOpen] = useState(false);
  const ctx = useCtxMenu();

  const list = useMemo(
    () =>
      workspace.functions.filter(
        (f) =>
          (kind === "all" || f.kind === kind) &&
          `${f.name} ${f.desc}`.toLowerCase().includes(q.toLowerCase()),
      ),
    [workspace.functions, q, kind],
  );

  const deleteFn = async (f: UiFunctionSummary) => {
    const ok = await ask({
      title: t("functions.deleteTitle"),
      description: t("functions.deleteDescription", { name: f.name }),
      confirmLabel: t("functions.deleteConfirm"),
      danger: true,
    });
    if (!ok) return;
    try {
      await workspace.deleteFunction(f.id);
      await workspace.refreshFunctions();
    } catch {
      workspace.notify(t("functionEditor.saveFailed"), "AlertTriangle");
    }
  };

  const fnMenu = (e: React.MouseEvent, f: UiFunctionSummary) =>
    ctx(e, [
      {
        label: t("functions.open"),
        icon: "ArrowUpRight",
        onSelect: () => void nav.openFunction(f),
      },
      ...(f.publishedRevision === 0
        ? [
            {
              label: t("functions.publish"),
              icon: "UploadCloud",
              onSelect: () => {
                void (async () => {
                  try {
                    const full = await workspace.getFunction(f.id);
                    await workspace.publishFunction(full);
                    await workspace.refreshFunctions();
                    workspace.notify(t("editor.published"), "UploadCloud");
                  } catch {
                    workspace.notify(t("functions.publishFailed"), "AlertTriangle");
                  }
                })();
              },
            },
          ]
        : []),
      { type: "sep" as const },
      {
        label: t("common.delete"),
        icon: "Trash2",
        danger: true,
        onSelect: () => void deleteFn(f),
      },
    ]);

  const create = async (req: { name: string; description: string; kind: "function" | "tool"; mode: "pure" | "impure" }) => {
    try {
      const created = await workspace.createFunction({
        name: req.name,
        description: req.description,
        kind: req.kind,
        mode: req.mode,
      });
      setCreateOpen(false);
      await workspace.refreshFunctions();
      if (created) await nav.openFunction(fnSummary(created));
    } catch {
      workspace.notify(t("functions.createFailed"), "AlertTriangle");
    }
  };

  return (
    <ViewShell
      title={t("functions.title")}
      subtitle={t("functions.description")}
      actions={
        <Button icon="Plus" variant="primary" onClick={() => setCreateOpen(true)}>
          {t("functions.new")}
        </Button>
      }
      toolbar={
        <>
          <SearchInput value={q} onChange={setQ} placeholder={t("functions.search")} className="w-[240px]" />
          <SegmentedControl
            value={kind}
            onChange={(v) => setKind(v)}
            segments={[
              { value: "all", label: t("common.all") },
              { value: "pure", label: t("functions.pure") },
              { value: "impure", label: t("functions.impure") },
              { value: "tool", label: t("functions.tool") },
            ]}
          />
          <span className="ml-auto text-[11.5px] text-fg-faint">{t("status.count", { count: list.length })}</span>
        </>
      }
    >
      {list.length === 0 ? (
        <EmptyState icon="SquareFunction" title={t("functions.emptyTitle")} hint={t("functions.emptyDescription")} />
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3">
          {list.map((f) => {
            const meta = FN_KIND_META[f.kind];
            return (
              <Card
                key={f.id}
                onClick={() => void nav.openFunction(f)}
                onContextMenu={(e) => fnMenu(e, f)}
                className="group flex flex-col p-3.5"
              >
                <div className="flex items-start gap-2.5">
                  <span className={cn("grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850", meta.tone)}>
                    <Icon name={meta.icon} className="h-4 w-4" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="flex items-center gap-1.5">
                      <span className="truncate text-[13px] font-medium text-fg">{f.name}</span>
                    </span>
                    <span className={cn("mt-1 inline-flex items-center gap-1 rounded px-1.5 py-px text-[10px] font-medium", meta.tone)}>
                      <Icon name={meta.icon} className="h-2.5 w-2.5" />
                      {t(meta.labelKey)}
                    </span>
                  </span>
                  <Icon name="ChevronRight" className="mt-1 h-4 w-4 shrink-0 text-fg-faint transition group-hover:text-fg-muted" />
                </div>

                <p className="mt-2.5 line-clamp-2 min-h-[32px] text-[11.5px] leading-relaxed text-fg-subtle">{f.desc}</p>

                <div className="mt-3 flex items-center gap-3 border-t border-seam pt-2.5 text-[10.5px] text-fg-faint">
                  {f.publishedRevision > 0 ? (
                    <span className="rounded bg-success/10 px-1.5 py-px text-success-fg">
                      {t("functions.published", { version: f.publishedRevision })}
                    </span>
                  ) : (
                    <span className="text-fg-subtle">{t("functions.draft")}</span>
                  )}
                  <span className="ml-auto">{f.updated}</span>
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {createOpen && (
        <FunctionCreateDialog
          onCancel={() => setCreateOpen(false)}
          onCreate={(req) => void create(req)}
        />
      )}
    </ViewShell>
  );
}

/** Builds a minimal summary for opening a freshly created function. */
function fnSummary(created: import("@/lib/types").CustomFunction): UiFunctionSummary {
  return {
    id: created.id,
    name: created.name,
    desc: created.description,
    kind: created.kind === "tool" ? "tool" : created.mode === "pure" ? "pure" : "impure",
    updated: formatDateTime(created.updatedAt),
    category: created.category,
    icon: created.icon || "Braces",
    publishedRevision: created.publishedRevision,
    pinsLoaded: false,
    inputs: [],
    outputs: [],
  };
}


