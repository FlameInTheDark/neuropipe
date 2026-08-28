import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import Editor from "@monaco-editor/react";
import type { Database, DataType, DatabaseTable, GlobalVariableSummary, SaveGlobalVariableRequest, SQLResult } from "@/lib/types";
import { desktop } from "@/lib/bridge";
import { formatDateTime, valuePreview } from "@/lib/format";
import type { Workspace } from "@/features/workspace/useWorkspace";
import { ask } from "@/stores/confirmation";
import { Card, EmptyState, SearchInput, StatusPill, ViewShell } from "../components/ViewShell";
import { Button } from "../components/ui";
import { Icon } from "../components/icons";
import { useCtxMenu } from "../components/ContextMenu";
import { ConnectionModal } from "../features/database/ConnectionModal";
import { KVBrowser } from "../features/database/KVBrowser";
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
          <div className="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1.6fr)_90px_150px_32px] items-center gap-3 border-b border-seam bg-ink-850/70 px-3 py-2 text-[10.5px] font-medium tracking-[0.08em] text-fg-subtle uppercase">
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
                <Icon name="Braces" className="h-3.5 w-3.5 shrink-0 text-fg-faint" />
                <span className="truncate font-mono text-[11.5px] text-fg">{v.name}</span>
              </span>
              <span className="truncate font-mono text-[11.5px] text-fg-subtle">{valuePreview(v.value)}</span>
              <span className="text-[11.5px] text-fg-subtle">{t(`variables.type.${v.dataType}`)}</span>
              <span className="truncate text-right text-[11px] text-fg-subtle">{formatDateTime(v.updatedAt)}</span>
              <button
                onClick={() => setEditing(v)}
                aria-label={t("common.edit")}
                className="grid h-6 w-6 place-items-center rounded text-fg-faint opacity-0 transition group-hover:opacity-100 hover:text-fg"
              >
                <Icon name="ChevronRight" className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
      )}
      <p className="mt-3 flex items-center gap-1.5 px-1 text-[11.5px] text-fg-faint">
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
              <div className="flex h-8 items-center rounded-md border border-ink-700 bg-ink-900 px-2.5 text-[12.5px] text-fg-subtle">
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


/** Quotes an identifier for the target dialect. */
function quoteIdent(driver: string, name: string): string {
  return driver === "mysql" ? "`" + name + "`" : '"' + name + '"';
}

function cellText(value: unknown): string {
  if (value === null || value === undefined) return "NULL";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

export function DatastoresView() {
  const { t } = useTranslation();
  const [databases, setDatabases] = useState<Database[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [schemaTables, setSchemaTables] = useState<DatabaseTable[] | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Database | null>(null);

  /* detail tabs: schema / data browser / SQL query */
  const [tab, setTab] = useState<"schema" | "data" | "query">("schema");
  const [expandedTable, setExpandedTable] = useState<string | null>(null);
  const [dataTable, setDataTable] = useState<string | null>(null);
  const [page, setPage] = useState(0);
const [pageSize, setPageSize] = useState(50);
  const [rowsPage, setRowsPage] = useState<SQLResult | null>(null);
  const [totalRows, setTotalRows] = useState<number | null>(null);
  const [loadingRows, setLoadingRows] = useState(false);
  const [rowsError, setRowsError] = useState<string | null>(null);
  const [queryText, setQueryText] = useState("");
  const [queryResult, setQueryResult] = useState<SQLResult | null>(null);
  const [queryError, setQueryError] = useState<string | null>(null);
  const [queryRunning, setQueryRunning] = useState(false);

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
    setSchemaTables(null);
    setExpandedTable(null);
    setDataTable(null);
    setPage(0);
    setRowsPage(null);
    setTotalRows(null);
    setRowsError(null);
    setQueryText("");
    setQueryResult(null);
    setQueryError(null);
    setTab("schema");
    if (!selected || selected.driver === "redis" || selected.driver === "sugardb") return;
    let cancelled = false;
    desktop
      .inspectDatabase(selected.id)
      .then((s) => !cancelled && setSchemaTables(s.tables))
      .catch(() => !cancelled && setSchemaTables([]));
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

  /** Loads one page of table rows plus a total count. */
  const loadRows = useCallback(
    async (page: number, pageSize: number) => {
      if (!selected || !dataTable) return;
      const quoted = quoteIdent(selected.driver, dataTable);
      setLoadingRows(true);
      setRowsError(null);
      try {
        const rows = await desktop.debugDatabase({
          databaseId: selected.id,
          sql: "SELECT * FROM " + quoted + " LIMIT " + pageSize + " OFFSET " + page * pageSize,
          parameters: [],
        });
        const count = await desktop.debugDatabase({
          databaseId: selected.id,
          sql: "SELECT COUNT(*) AS n FROM " + quoted,
          parameters: [],
        });
        const n = Number(count.rows[0]?.n);
        setRowsPage(rows);
        setTotalRows(Number.isFinite(n) ? n : null);
      } catch (e) {
        setRowsPage(null);
        setTotalRows(null);
        setRowsError(e instanceof Error ? e.message : String(e));
      } finally {
        setLoadingRows(false);
      }
    },
    [selected, dataTable],
  );

  const runQuery = async () => {
    if (!selected || !queryText.trim() || queryRunning) return;
    setQueryRunning(true);
    setQueryError(null);
    try {
      setQueryResult(await desktop.debugDatabase({ databaseId: selected.id, sql: queryText, parameters: [] }));
    } catch (e) {
      setQueryResult(null);
      setQueryError(e instanceof Error ? e.message : String(e));
    } finally {
      setQueryRunning(false);
    }
  };

  /** Opens a table in the Data tab (used by schema rows). */
  const openTableData = (name: string) => {
    if (!selected) return;
    setDataTable(name);
    setPage(0);
    setTab("data");
    void loadRowsRef.current(0, pageSize);
  };

  const loadRowsRef = useRef(loadRows);
  useEffect(() => {
    loadRowsRef.current = loadRows;
  }, [loadRows]);

  /* fetch rows whenever the Data tab's page, page size or table changes */
  useEffect(() => {
    if (tab !== "data" || !dataTable) return;
    void loadRowsRef.current(page, pageSize);
  }, [tab, dataTable, page, pageSize]);

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
                <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg-subtle">
                  <Icon name="Database" className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[12.5px] font-medium text-fg">{d.name}</span>
                  <span className="block truncate text-[11px] capitalize text-fg-faint">{d.driver}</span>
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
            className="mt-1 flex w-full items-center gap-2 rounded-lg border border-dashed border-ink-700 px-2.5 py-2 text-[11.5px] text-fg-faint transition hover:border-ink-500 hover:text-fg-muted"
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
        <div className="fade-in flex min-h-0 flex-1 flex-col gap-4 overflow-hidden p-4">
          <>
            <div className="flex items-start gap-3">
                <span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-ink-700 bg-ink-850 text-fg">
                  <Icon name="Database" className="h-[18px] w-[18px]" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <h2 className="truncate text-[15px] font-semibold text-fg">{selected.name}</h2>
                    <StatusPill status={selected.status === "connected" ? "connected" : selected.status === "error" ? "error" : "idle"} />
                  </div>
                  <p className="mt-0.5 truncate font-mono text-[11px] text-fg-faint">
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
                  className="hover:bg-danger/20 hover:text-danger-fg"
                  onClick={() => void removeDb(selected)}
                >
                  {t("databases.unregister")}
                </Button>
              </div>

              <div className="mt-4 grid grid-cols-4 gap-2.5">
                {[
                  [t("datastores.driver"), selected.driver.toUpperCase()],
                  selected.driver === "redis" || selected.driver === "sugardb"
                    ? [t("databases.dbIndex"), `db${selected.dbIndex ?? 0}`]
                    : [t("datastores.tables"), String(schemaTables?.length ?? "…")],
                  [
                    t("datastores.updated"),
                    formatDateTime(selected.updatedAt),
                  ],
                  [t("datastores.lastPing"), formatDateTime(selected.lastPingAt)],
                ].map(([k, v]) => (
                  <Card key={k} className="p-3">
                    <span className="text-[10px] tracking-wide text-fg-faint uppercase">{k}</span>
                    <p className="mt-1 truncate text-[13px] font-semibold capitalize text-fg">{v}</p>
                  </Card>
                ))}
              </div>

              {selected.driver === "redis" || selected.driver === "sugardb" ? (
                <KVBrowser database={selected} />
              ) : (
              <>
              {/* tabs: schema / data / query */}
              <div className="mt-5 flex items-center gap-2">
                <div className="flex items-center gap-0.5 rounded-lg border border-ink-700 bg-ink-900 p-0.5">
                  {([
                    { id: "schema", label: t("sql.schema"), icon: "List" },
                    { id: "data", label: t("datastores.tabData"), icon: "Table2" },
                    { id: "query", label: t("datastores.tabQuery"), icon: "Terminal" },
                  ] as const).map((tb) => (
                    <button
                      key={tb.id}
                      onClick={() => setTab(tb.id)}
                      aria-pressed={tab === tb.id}
                      className={cn(
                        "flex h-7 items-center gap-1.5 rounded-md px-3 text-[11.5px] transition",
                        tab === tb.id ? "bg-ink-700 text-fg" : "text-fg-subtle hover:text-fg",
                      )}
                    >
                      <Icon name={tb.icon} className="h-3 w-3" />
                      {tb.label}
                    </button>
                  ))}
                </div>
                {tab === "data" && schemaTables !== null && schemaTables.length > 0 && (
                  <Dropdown
                    value={dataTable ?? ""}
                    onChange={(name) => {
                      setDataTable(name || null);
                      setPage(0);
                    }}
                    className="w-[220px]"
                    placeholder={t("datastores.pickTable")}
                    options={schemaTables.map((tb) => ({ value: tb.name, label: tb.name, icon: "Table2" }))}
                  />
                )}
                {tab === "data" && (
                  <Button
                    variant="ghost"
                    icon="RefreshCw"
                    disabled={loadingRows}
                    onClick={() => {
                      if (!selected) return;
                      void desktop
                        .inspectDatabase(selected.id)
                        .then((s) => setSchemaTables(s.tables))
                        .catch(() => undefined);
                      if (dataTable) void loadRowsRef.current(page, pageSize);
                    }}
                  >
                    {t("common.refresh")}
                  </Button>
                )}
              </div>

              {tab === "schema" && (
                <div className="overflow-hidden rounded-xl border border-ink-700/80">
                  {(schemaTables ?? []).map((tb, i) => {
                    const expanded = expandedTable === tb.name;
                    return (
                      <div key={tb.name} className={cn("bg-ink-850/50", i > 0 && "border-t border-seam")}>
                        <button
                          onClick={() => setExpandedTable(expanded ? null : tb.name)}
                          className="flex w-full items-center gap-2.5 px-3 py-2 text-left transition hover:bg-ink-850"
                        >
                          <Icon name="ChevronRight" className={cn("h-3 w-3 shrink-0 text-fg-faint transition-transform", expanded && "rotate-90")} />
                          <Icon name="Table2" className="h-3.5 w-3.5 shrink-0 text-fg-faint" />
                          <span className="font-mono text-[11.5px] text-fg">{tb.name}</span>
                          <span className="ml-auto font-mono text-[10.5px] text-fg-faint">
                            {t("datastores.columns", { count: tb.columns.length })}
                          </span>
                        </button>
                        {expanded && (
                          <div className="border-t border-seam/70 bg-ink-950/40 px-6 py-2">
                            <p className="mb-1 text-[10px] uppercase tracking-[0.09em] text-fg-faint">{t("datastores.columnsTitle")}</p>
                            {tb.columns.map((col) => (
                              <div key={col.name} className="flex items-baseline gap-2 py-0.5">
                                <span className="font-mono text-[11px] text-fg">{col.name}</span>
                                <span className="font-mono text-[10.5px] text-info-fg/70">{col.dataType}</span>
                                {col.primaryKey && (
                                  <span className="rounded bg-warning/15 px-1 font-mono text-[9.5px] text-warning-fg">PK</span>
                                )}
                                {!col.nullable && <span className="text-[9.5px] text-fg-faint">NOT NULL</span>}
                                {col.default && <span className="truncate font-mono text-[10px] text-fg-faint">= {col.default}</span>}
                              </div>
                            ))}
                            {tb.indexes.length > 0 && (
                              <>
                                <p className="mb-1 mt-2 text-[10px] uppercase tracking-[0.09em] text-fg-faint">{t("datastores.indexes")}</p>
                                {tb.indexes.map((ix) => (
                                  <div key={ix.name} className="flex items-baseline gap-2 py-0.5">
                                    <span className="font-mono text-[11px] text-fg-muted">{ix.name}</span>
                                    {ix.unique && <span className="text-[9.5px] text-violet-300/80">UNIQUE</span>}
                                    <span className="truncate font-mono text-[10px] text-fg-faint">({ix.columns.join(", ")})</span>
                                  </div>
                                ))}
                              </>
                            )}
                            <Button variant="ghost" icon="Table2" className="mt-2" onClick={() => { openTableData(tb.name); }}>
                              {t("datastores.viewData")}
                            </Button>
                          </div>
                        )}
                      </div>
                    );
                  })}
                  {schemaTables !== null && schemaTables.length === 0 && (
                    <p className="bg-ink-850/40 px-3 py-3 text-[12px] text-fg-faint">{t("sql.noTables")}</p>
                  )}
                </div>
              )}

              {tab === "data" && (
                dataTable ? (
                  <div className="min-h-0 flex-1 overflow-hidden rounded-xl border border-ink-700/80 bg-ink-900/60 flex flex-col">
                    <div className="flex items-center gap-2 border-b border-seam px-3 py-1.5 text-[11px] text-fg-subtle">
                      <Icon name="Table2" className="h-3.5 w-3.5 text-fg-faint" />
                      <span className="font-mono text-[11.5px] text-fg-muted">{dataTable}</span>
                      <span className="ml-auto font-mono text-[10.5px]">
                        {loadingRows ? t("common.loading") : totalRows !== null ? t("datastores.rowsTotal", { count: totalRows }) : ""}
                      </span>
                    </div>
                    <div className="min-h-0 flex-1 overflow-auto">
                      {rowsError ? (
                        <p className="flex items-start gap-2 bg-danger/10 px-4 py-3 text-[11.5px] text-danger-fg">
                          <Icon name="AlertTriangle" className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                          {rowsError}
                        </p>
                      ) : rowsPage && rowsPage.columns.length > 0 ? (
                        <table className="w-full border-collapse text-left">
                          <thead className="sticky top-0 bg-ink-900">
                            <tr className="border-b border-seam">
                              {rowsPage.columns.map((c) => (
                                <th key={c} className="whitespace-nowrap px-3 py-1.5 font-mono text-[10.5px] font-medium text-fg-subtle">{c}</th>
                              ))}
                            </tr>
                          </thead>
                          <tbody>
                            {rowsPage.rows.map((row, i) => (
                              <tr key={i} className="border-b border-seam/60 last:border-b-0 hover:bg-ink-850/60">
                                {rowsPage.columns.map((c) => (
                                  <td key={c} className="max-w-[280px] truncate whitespace-nowrap px-3 py-1 font-mono text-[10.5px] text-fg-muted">
                                    {cellText(row[c])}
                                  </td>
                                ))}
                              </tr>
                            ))}
                            {rowsPage.rows.length === 0 && (
                              <tr><td colSpan={Math.max(rowsPage.columns.length, 1)} className="px-3 py-2 text-[11.5px] text-fg-faint">{t("sql.noResult")}</td></tr>
                            )}
                          </tbody>
                        </table>
                      ) : null}
                    </div>
                    <div className="flex h-9 shrink-0 items-center gap-2 border-t border-seam px-3 text-[11px] text-fg-faint">
                      <select
                        value={pageSize}
                        onChange={(e) => {
                          setPage(0);
                          setPageSize(Number(e.target.value));
                        }}
                        aria-label={t("datastores.perPage", { count: pageSize })}
                        className="h-6 rounded-md border border-ink-700 bg-ink-850 px-1 font-mono text-[10.5px] text-fg-muted [color-scheme:dark] focus:border-ink-500 focus:outline-none"
                      >
                        {[25, 50, 100, 200].map((n) => (
                          <option key={n} value={n}>{n}</option>
                        ))}
                      </select>
                      <Button variant="ghost" icon="ChevronLeft" disabled={page === 0 || loadingRows}
                        onClick={() => setPage((p) => Math.max(0, p - 1))}>
                        {t("datastores.prevPage")}
                      </Button>
                      <span className="shrink-0">{t("datastores.page")}</span>
                      <input
                        type="number"
                        min={1}
                        value={page + 1}
                        onChange={(e) => {
                          const next = Number(e.target.value);
                          if (Number.isFinite(next) && next >= 1) setPage(next - 1);
                        }}
                        aria-label={t("datastores.page")}
                        className="h-6 w-[52px] rounded-md border border-ink-700 bg-ink-850 px-1.5 text-center font-mono text-[10.5px] tabular-nums text-fg-muted [color-scheme:dark] focus:border-ink-500 focus:outline-none"
                      />
                      <Button variant="ghost" icon="ChevronRight"
                        disabled={loadingRows || (totalRows !== null && (page + 1) * pageSize >= totalRows)}
                        onClick={() => setPage((p) => p + 1)}>
                        {t("datastores.nextPage")}
                      </Button>
                      {totalRows !== null && (
                        <span className="ml-auto shrink-0 font-mono text-[10.5px]">
                          {t("datastores.rowsTotal", { count: totalRows })}
                        </span>
                      )}
                    </div>
                  </div>
                ) : (
                  <div className="grid flex-1 place-items-center overflow-hidden rounded-xl border border-ink-700/80">
                    <EmptyState icon="Table2" title={t("datastores.pickTable")} hint={t("sql.noTables")} />
                  </div>
                )
              )}

              {tab === "query" && (
                <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-hidden rounded-xl border border-ink-700/80 bg-ink-900/60 p-2">
                  <div className="relative min-h-[160px] flex-1 overflow-hidden rounded-lg border border-ink-700 bg-ink-950">
                    <Editor
                      language="sql"
                      value={queryText}
                      onChange={(v) => setQueryText(v ?? "")}
                      theme="vs-dark"
                      options={{
                        minimap: { enabled: false },
                        fontSize: 12.5,
                        lineNumbers: "on",
                        scrollBeyondLastLine: false,
                        wordWrap: "on",
                        automaticLayout: true,
                        tabSize: 2,
                      }}
                      loading={<span className="absolute inset-0 grid place-items-center text-[12px] text-fg-faint">{t("common.loading")}</span>}
                    />
                  </div>
                  {queryError && (
                    <p className="flex items-start gap-2 rounded-lg bg-danger/10 px-3 py-2 text-[11.5px] leading-relaxed text-danger-fg">
                      <Icon name="AlertTriangle" className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                      {queryError}
                    </p>
                  )}
                  {queryResult && queryResult.columns.length > 0 && (
                    <div className="max-h-[240px] overflow-auto rounded-lg border border-ink-700">
                      <table className="w-full border-collapse text-left">
                        <thead>
                          <tr className="border-b border-seam bg-ink-850/60">
                            {queryResult.columns.map((c) => (
                              <th key={c} className="whitespace-nowrap px-3 py-1.5 font-mono text-[10.5px] font-medium text-fg-subtle">{c}</th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {queryResult.rows.map((row, i) => (
                            <tr key={i} className="border-b border-seam/60 hover:bg-ink-850/60">
                              {queryResult.columns.map((c) => (
                                <td key={c} className="max-w-[280px] truncate whitespace-nowrap px-3 py-1 font-mono text-[10.5px] text-fg-muted">
                                  {cellText(row[c])}
                                </td>
                              ))}
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                  <div className="flex items-center justify-end gap-2">
                    <Button variant="primary" icon={queryRunning ? "Loader2" : "Play"} spin={queryRunning} disabled={queryRunning || !queryText.trim()} onClick={() => void runQuery()}>
                      {queryRunning ? t("common.loading") : t("sql.run")}
                    </Button>
                  </div>
                </div>
              )}
              </>
              )}
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
        status === "connected" ? "bg-success" : status === "error" ? "bg-danger" : "bg-ink-500",
      )}
    />
  );
}

function detailLine(db: Database): string {
  if (db.driver === "sqlite") return db.path ?? "";
  if (db.driver === "sugardb") {
    if (db.path) return `${db.path}/db${db.dbIndex ?? 0}`;
    return `embedded · app data/db${db.dbIndex ?? 0}`;
  }
  if (db.driver === "redis") {
    if (db.address) return db.address;
    return `${db.host}:${db.port ?? 6379}/db${db.dbIndex ?? 0}`;
  }
  return `${db.host}:${db.port ?? (db.driver === "postgres" ? 5432 : 3306)}/${db.database ?? ""}`;
}

/* ---------- database create/edit dialog ---------- */




