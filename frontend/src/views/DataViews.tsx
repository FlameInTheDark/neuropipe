import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { Database, DataType, GlobalVariableSummary, SaveGlobalVariableRequest } from "@/lib/types";
import { desktop } from "@/lib/bridge";
import { formatDateTime, valuePreview } from "@/lib/format";
import type { Workspace } from "@/features/workspace/useWorkspace";
import { ask } from "@/stores/confirmation";
import { Card, EmptyState, SearchInput, StatusPill, ViewShell } from "../components/ViewShell";
import { Button } from "../components/ui";
import { Icon } from "../components/icons";
import { useCtxMenu } from "../components/ContextMenu";
import { ConnectionModal } from "../features/database/ConnectionModal";
import { Modal, ModalActions } from "../components/primitives/Modal";
import { Field, TextArea, TextInput } from "../components/primitives/Field";
import { Dropdown } from "../components/Dropdown";
import { cn } from "../utils/cn";

/* ---------------- Variables ---------------- */

const VAR_TYPES: DataType[] = ["text", "number", "boolean", "object", "list", "any"];

export function VariablesView({ workspace }: { workspace: Workspace }) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [editing, setEditing] = useState<GlobalVariableSummary | null>(null);
  const [creating, setCreating] = useState(false);
  const ctx = useCtxMenu();

  const list = workspace.variables.filter((v) => v.name.toLowerCase().includes(q.toLowerCase()));

  const onVarCtx = (e: React.MouseEvent, v: GlobalVariableSummary) =>
    ctx(e, [
      { label: t("common.edit"), icon: "Pencil", onSelect: () => setEditing(v) },
      {
        label: t("variables.copyValue"),
        icon: "Copy",
        hint: "⌘C",
        onSelect: () => navigator.clipboard?.writeText(valuePreview(v.value)),
      },
      { type: "sep" },
      {
        label: t("common.delete"),
        icon: "Trash2",
        danger: true,
        onSelect: async () => {
          const ok = await ask({
            title: t("variables.deleteTitle"),
            description: t("variables.deleteDescription", { name: v.name }),
            confirmLabel: t("variables.deleteConfirm"),
            danger: true,
          });
          if (!ok) return;
          await workspace.deleteVariable(v.id);
        },
      },
    ]);

  return (
    <ViewShell
      title={t("variables.title")}
      subtitle={t("status.count", { count: workspace.variables.length })}
      actions={
        <Button icon="Plus" variant="primary" onClick={() => setCreating(true)}>
          {t("variables.new")}
        </Button>
      }
      toolbar={<SearchInput value={q} onChange={setQ} placeholder={t("variables.search")} className="w-[260px]" />}
    >
      {list.length === 0 ? (
        <EmptyState icon="Braces" title={t("variables.emptyTitle")} hint={t("variables.emptyDescription")} />
      ) : (
        <div className="overflow-hidden rounded-xl border border-ink-700/80">
          <div className="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1.6fr)_90px_150px_32px] items-center gap-3 border-b border-seam bg-ink-850/70 px-3 py-2 text-[10.5px] font-medium tracking-[0.08em] text-ink-400 uppercase">
            <span>{t("variables.nameLabel")}</span>
            <span>{t("inputDialog.value")}</span>
            <span>{t("variables.typeLabel")}</span>
            <span className="text-right">{t("pipelines.updated")}</span>
            <span />
          </div>
          {list.map((v) => (
            <div
              key={v.id}
              onContextMenu={(e) => onVarCtx(e, v)}
              onDoubleClick={() => setEditing(v)}
              className="group grid cursor-default grid-cols-[minmax(0,1.1fr)_minmax(0,1.6fr)_90px_150px_32px] items-center gap-3 border-b border-seam/70 px-3 py-2 transition last:border-b-0 hover:bg-ink-850"
            >
              <span className="flex min-w-0 items-center gap-2">
                <Icon name="Braces" className="h-3.5 w-3.5 shrink-0 text-ink-500" />
                <span className="truncate font-mono text-[11.5px] text-ink-50">{v.name}</span>
              </span>
              <span className="truncate font-mono text-[11.5px] text-ink-300">{valuePreview(v.value)}</span>
              <span className="text-[11.5px] text-ink-400">{t(`variables.type.${v.dataType}`)}</span>
              <span className="truncate text-right text-[11px] text-ink-400">{formatDateTime(v.updatedAt)}</span>
              <button
                onClick={() => setEditing(v)}
                aria-label={t("common.edit")}
                className="grid h-6 w-6 place-items-center rounded text-ink-600 opacity-0 transition group-hover:opacity-100 hover:text-ink-100"
              >
                <Icon name="ChevronRight" className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
      )}
      <p className="mt-3 flex items-center gap-1.5 px-1 text-[11.5px] text-ink-500">
        <Icon name="Info" className="h-3.5 w-3.5" />
        {t("variables.referenceHint")}
      </p>

      {(creating || editing) && (
        <VariableDialog
          existing={editing}
          onCancel={() => {
            setCreating(false);
            setEditing(null);
          }}
          onSave={async (req) => {
            try {
              if (editing) await workspace.updateVariable({ ...req, id: editing.id });
              else await workspace.createVariable(req);
              setCreating(false);
              setEditing(null);
            } catch {
              workspace.notify(t("variables.saveFailed"), "AlertTriangle");
            }
          }}
        />
      )}
    </ViewShell>
  );
}

function VariableDialog({
  existing,
  onSave,
  onCancel,
}: {
  existing: GlobalVariableSummary | null;
  onSave: (req: SaveGlobalVariableRequest) => void | Promise<void>;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState(existing?.name ?? "");
  const [description, setDescription] = useState(existing?.description ?? "");
  const [dataType, setDataType] = useState<DataType>(existing?.dataType ?? "text");
  const [defaultValue, setDefaultValue] = useState(stringifyDefault(existing?.value));

  const validName = /^[A-Za-z_][A-Za-z0-9_]*$/.test(name);
  const jsonValid =
    (dataType !== "object" && dataType !== "list") ||
    (() => {
      try {
        const parsed = JSON.parse(defaultValue);
        if (dataType === "object") return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed);
        return Array.isArray(parsed);
      } catch {
        return false;
      }
    })();

  const canSave = Boolean(name.trim()) && validName !== false && jsonValid;

  const buildRequest = (): SaveGlobalVariableRequest => ({
    ...(existing ? { id: existing.id } : {}),
    name: existing ? existing.name : name.trim(),
    description,
    dataType,
    defaultValue: parseDefault(defaultValue, dataType),
  });

  return (
    <Modal
      title={existing ? t("variables.editTitle") : t("variables.createTitle")}
      icon="Braces"
      onClose={onCancel}
      footer={<ModalActions onCancel={onCancel} onConfirm={() => void onSave(buildRequest())} disabled={!canSave} />}
    >
      <div className="space-y-3">
        <Field label={t("variables.nameLabel")} required hint={!validName && name ? t("variables.nameInvalid") : undefined}>
          <TextInput value={name} onChange={setName} placeholder={t("variables.namePlaceholder")} mono disabled={Boolean(existing)} />
        </Field>
        <Field label={t("variables.descriptionLabel")}>
          <TextArea value={description} onChange={setDescription} placeholder={t("variables.descriptionPlaceholder")} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label={t("variables.typeLabel")} hint={existing ? t("variables.typeFrozen") : undefined}>
            {existing ? (
              <div className="flex h-8 items-center rounded-md border border-ink-700 bg-ink-900 px-2.5 text-[12.5px] text-ink-400">
                {t(`variables.type.${dataType}`)}
              </div>
            ) : (
              <Dropdown
                value={dataType}
                onChange={(v) => setDataType(v as DataType)}
                options={VAR_TYPES.map((v) => ({ value: v, label: t(`variables.type.${v}`) }))}
              />
            )}
          </Field>
          <Field label={t("variables.defaultLabel")} hint={!jsonValid ? t("variables.defaultInvalidJson") : undefined}>
            {dataType === "boolean" ? (
              <Dropdown
                value={defaultValue}
                onChange={setDefaultValue}
                options={[
                  { value: "true", label: t("variables.booleanTrue") },
                  { value: "false", label: t("variables.booleanFalse") },
                ]}
              />
            ) : dataType === "object" || dataType === "list" ? (
              <TextArea rows={3} value={defaultValue} onChange={setDefaultValue} className="font-mono text-[11.5px]" />
            ) : (
              <TextInput value={defaultValue} onChange={setDefaultValue} placeholder={t("variables.defaultPlaceholder")} />
            )}
          </Field>
        </div>
      </div>
    </Modal>
  );
}

function stringifyDefault(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  if (typeof value === "boolean" || typeof value === "number") return String(value);
  try {
    return JSON.stringify(value);
  } catch {
    return "";
  }
}

function parseDefault(raw: string, dataType: DataType): unknown {
  switch (dataType) {
    case "boolean":
      return raw.trim().toLowerCase() === "true";
    case "number": {
      const n = Number(raw);
      return Number.isFinite(n) ? n : raw;
    }
    default:
      if (!raw.trim()) return "";
      try {
        return JSON.parse(raw);
      } catch {
        return raw;
      }
  }
}

/* ---------------- Datastores ---------------- */

export function DatastoresView() {
  const { t } = useTranslation();
  const [databases, setDatabases] = useState<Database[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [schema, setSchema] = useState<{ name: string; columns: number }[] | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Database | null>(null);

  const load = async () => {
    const list = await desktop.listDatabases();
    setDatabases(list);
    setSelectedId((prev) => prev ?? list[0]?.id ?? null);
  };

  useEffect(() => {
    void load();
  }, []);

  const selected = databases.find((d) => d.id === selectedId) ?? databases[0];

  useEffect(() => {
    setSchema(null);
    if (!selected) return;
    let cancelled = false;
    desktop
      .inspectDatabase(selected.id)
      .then((s) => !cancelled && setSchema(s.tables.map((tb) => ({ name: tb.name, columns: tb.columns.length }))))
      .catch(() => !cancelled && setSchema([]));
    return () => {
      cancelled = true;
    };
  }, [selected]);

  const ping = async () => {
    if (!selected) return;
    try {
      await desktop.pingDatabase(selected.id);
      await load();
    } catch {
      /* status pill reflects the failure */
    }
  };

  const removeDb = async (db: Database) => {
    const ok = await ask({
      title: t("databases.unregisterTitle"),
      description: t("databases.unregisterDescription", { name: db.name }),
      confirmLabel: t("databases.unregister"),
      danger: true,
    });
    if (!ok) return;
    await desktop.deleteDatabase(db.id);
    setSelectedId(null);
    await load();
  };

  return (
    <ViewShell
      title={t("datastores.title")}
      subtitle={t("datastores.description")}
      padded={false}
      actions={
        <Button
          icon="Plus"
          variant="primary"
          onClick={() => {
            setEditing(null);
            setDialogOpen(true);
          }}
        >
          {t("dbnew.newConnection")}
        </Button>
      }
    >
      <div className="flex h-full min-h-0">
        <div className="w-[280px] shrink-0 overflow-y-auto border-r border-seam p-2.5">
          {databases.length === 0 ? (
            <EmptyState icon="Database" title={t("databases.emptyTitle")} hint={t("databases.emptyDescription")} />
          ) : (
            databases.map((d) => (
              <button
                key={d.id}
                onClick={() => setSelectedId(d.id)}
                className={cn(
                  "mb-1.5 flex w-full items-center gap-2.5 rounded-lg border px-2.5 py-2 text-left transition",
                  selected?.id === d.id
                    ? "border-ink-500 bg-ink-800/70"
                    : "border-transparent hover:border-ink-700 hover:bg-ink-850",
                )}
              >
                <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-ink-300">
                  <Icon name="Database" className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[12.5px] font-medium text-ink-50">{d.name}</span>
                  <span className="block truncate text-[11px] capitalize text-ink-500">{d.driver}</span>
                </span>
                <StatusDot status={d.status} />
              </button>
            ))
          )}

          <button
            onClick={() => {
              setEditing(null);
              setDialogOpen(true);
            }}
            className="mt-1 flex w-full items-center gap-2 rounded-lg border border-dashed border-ink-700 px-2.5 py-2 text-[11.5px] text-ink-500 transition hover:border-ink-500 hover:text-ink-200"
          >
            <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850">
              <Icon name="Plus" className="h-4 w-4" />
            </span>
            {t("dbnew.newConnection")}
          </button>
        </div>

        {!selected ? (
          <div className="flex min-w-0 flex-1 items-center justify-center">
            <EmptyState icon="Database" title={t("dbnew.noSelectionTitle")} hint={t("dbnew.noSelectionHint")} />
          </div>
        ) : (
        <div className="fade-in min-w-0 flex-1 overflow-y-auto p-4">
          <>
            <div className="flex items-start gap-3">
                <span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-ink-700 bg-ink-850 text-ink-100">
                  <Icon name="Database" className="h-[18px] w-[18px]" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <h2 className="truncate text-[15px] font-semibold text-ink-50">{selected.name}</h2>
                    <StatusPill status={selected.status === "connected" ? "connected" : selected.status === "error" ? "error" : "idle"} />
                  </div>
                  <p className="mt-0.5 truncate font-mono text-[11px] text-ink-500">
                    {detailLine(selected)}
                  </p>
                </div>
                <Button icon="RefreshCw" onClick={() => void ping()}>
                  {t("datastores.reconnect")}
                </Button>
                <Button
                  variant="solid"
                  icon="Pencil"
                  onClick={() => {
                    setEditing(selected);
                    setDialogOpen(true);
                  }}
                >
                  {t("common.edit")}
                </Button>
                <Button
                  variant="solid"
                  icon="Trash2"
                  className="hover:bg-rose-500/20 hover:text-rose-200"
                  onClick={() => void removeDb(selected)}
                >
                  {t("databases.unregister")}
                </Button>
              </div>

              <div className="mt-4 grid grid-cols-4 gap-2.5">
                {[
                  [t("datastores.driver"), selected.driver.toUpperCase()],
                  [t("datastores.tables"), String(schema?.length ?? "…")],
                  [
                    t("datastores.updated"),
                    formatDateTime(selected.updatedAt),
                  ],
                  [t("datastores.lastPing"), formatDateTime(selected.lastPingAt)],
                ].map(([k, v]) => (
                  <Card key={k} className="p-3">
                    <span className="text-[10px] tracking-wide text-ink-500 uppercase">{k}</span>
                    <p className="mt-1 truncate text-[13px] font-semibold capitalize text-ink-50">{v}</p>
                  </Card>
                ))}
              </div>

              <h3 className="mt-5 mb-2 text-[10.5px] font-medium tracking-[0.09em] text-ink-400 uppercase">
                {t("sql.schema")}
              </h3>
              <div className="overflow-hidden rounded-xl border border-ink-700/80">
                {(schema ?? []).map((tb, i) => (
                  <div
                    key={tb.name}
                    className={cn(
                      "flex items-center gap-2.5 bg-ink-850/50 px-3 py-2 transition hover:bg-ink-850",
                      i > 0 && "border-t border-seam",
                    )}
                  >
                    <Icon name="Table2" className="h-3.5 w-3.5 text-ink-500" />
                    <span className="font-mono text-[11.5px] text-ink-100">{tb.name}</span>
                    <span className="ml-auto font-mono text-[10.5px] text-ink-500">
                      {t("datastores.columns", { count: tb.columns })}
                    </span>
                  </div>
                ))}
                {schema !== null && schema.length === 0 && (
                  <p className="bg-ink-850/40 px-3 py-3 text-[12px] text-ink-500">{t("sql.noTables")}</p>
                )}
              </div>
            </>
        </div>
        )}
      </div>

      {dialogOpen && (
        <ConnectionModal
          existing={editing}
          onClose={() => setDialogOpen(false)}
          onSaved={() => {
            setDialogOpen(false);
            setSelectedId(null);
            void load();
          }}
        />
      )}
    </ViewShell>
  );
}

function StatusDot({ status }: { status: string }) {
  return (
    <span
      className={cn(
        "h-1.5 w-1.5 shrink-0 rounded-full",
        status === "connected" ? "bg-emerald-400" : status === "error" ? "bg-rose-400" : "bg-ink-500",
      )}
    />
  );
}

function detailLine(db: Database): string {
  if (db.driver === "sqlite") return db.path ?? "";
  return `${db.host}:${db.port ?? (db.driver === "postgres" ? 5432 : 3306)}/${db.database ?? ""}`;
}

/* ---------- database create/edit dialog ---------- */




