import { useEffect, useState } from "react";
import { DatabaseIcon, FolderOpen, Loader2, MoreHorizontal, Pencil, Plug, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { ContextMenu, contextMenuPointFromElement, contextMenuPosition, type ContextMenuPosition } from "@/components/ContextMenu";
import { EmptyState } from "@/components/EmptyState";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { desktop } from "@/lib/bridge";
import type { Database, DatabaseDriver, DatabaseStatus, SaveDatabaseRequest } from "@/lib/types";
import { cn, formatDate } from "@/lib/utils";
import { useConfirmationStore } from "@/stores/confirmation";
import { useUIStore } from "@/stores/ui";

type DatabaseMode = "create" | "edit";

interface DriverBadge {
  label: string;
  className: string;
}

function driverBadge(driver: DatabaseDriver, t: (key: string) => string): DriverBadge {
  switch (driver) {
    case "sqlite":
      return { label: t("databases.driverSqlite"), className: "border-blue-500/30 bg-blue-500/15 text-blue-300" };
    case "postgres":
      return { label: t("databases.driverPostgres"), className: "border-indigo-500/30 bg-indigo-500/15 text-indigo-300" };
    case "mysql":
      return { label: t("databases.driverMysql"), className: "border-amber-500/30 bg-amber-500/15 text-amber-300" };
  }
}

function statusDotClass(status: DatabaseStatus): string {
  switch (status) {
    case "connected":
      return "bg-emerald-400";
    case "error":
      return "bg-red-400";
    case "unverified":
    case "unknown":
    default:
      return "bg-zinc-500";
  }
}

function statusLabel(status: DatabaseStatus, t: (key: string) => string): string {
  switch (status) {
    case "connected":
      return t("databases.statusConnected");
    case "error":
      return t("databases.statusError");
    case "unverified":
      return t("databases.statusUnverified");
    case "unknown":
    default:
      return t("databases.statusUnknown");
  }
}

function driverDetail(item: Database): string {
  if (item.driver === "sqlite") return item.path ?? "";
  const host = item.host ?? "localhost";
  const port = item.port ?? (item.driver === "postgres" ? 5432 : 3306);
  const database = item.database ?? "";
  return database ? `${host}:${port}/${database}` : `${host}:${port}`;
}

function defaultPort(driver: DatabaseDriver): number {
  if (driver === "postgres") return 5432;
  if (driver === "mysql") return 3306;
  return 0;
}

interface TestResult {
  ok: boolean;
  message: string;
}

function DatabaseDialog({
  mode,
  item,
  pending,
  onClose,
  onSaved,
}: {
  mode: DatabaseMode;
  item?: Database;
  pending: boolean;
  onClose: () => void;
  onSaved: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const setError = useUIStore((state) => state.setError);
  const isEdit = mode === "edit";

  const [name, setName] = useState(item?.name ?? "");
  const [driver, setDriver] = useState<DatabaseDriver>(item?.driver ?? "sqlite");

  // SQLite fields
  const [path, setPath] = useState(item?.path ?? "");
  const [createNew, setCreateNew] = useState(true);

  // Shared network fields (postgres / mysql)
  const [host, setHost] = useState(item?.host ?? "localhost");
  const [port, setPort] = useState<string>(item?.port !== undefined && item.port !== 0 ? String(item.port) : "");
  const [databaseName, setDatabaseName] = useState(item?.database ?? "");
  const [username, setUsername] = useState(item?.username ?? "");
  const [password, setPassword] = useState("");

  // PostgreSQL-specific
  const [schema, setSchema] = useState(item?.schema ?? "public");
  const [sslMode, setSslMode] = useState(item?.sslMode ?? "prefer");

  // MySQL-specific
  const [charset, setCharset] = useState(item?.charset ?? "utf8mb4");

  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<TestResult>();

  // When the driver changes while creating, reset network fields to sensible defaults.
  const onDriverChange = (next: string) => {
    const driver = next as DatabaseDriver;
    setDriver(driver);
    if (driver !== "sqlite") {
      const defaultP = defaultPort(driver);
      setPort((current) => (current === "" || current === "0" ? String(defaultP) : current));
    }
  };

  const buildRequest = (): SaveDatabaseRequest => {
    const request: SaveDatabaseRequest = {
      id: isEdit ? item?.id : undefined,
      name: name.trim(),
      driver,
    };
    if (driver === "sqlite") {
      request.path = path.trim();
    } else {
      request.host = host.trim();
      const portNumber = Number(port);
      request.port = Number.isFinite(portNumber) && portNumber > 0 ? portNumber : defaultPort(driver);
      request.database = databaseName.trim();
      request.username = username.trim();
      // On edit, keep the existing passwordRef so the backend can preserve the
      // stored secret when the user leaves the password blank. Only send the
      // new plaintext password when the user typed one.
      if (isEdit && item?.passwordRef) request.passwordRef = item.passwordRef;
      if (password) request.password = password;
      if (driver === "postgres") {
        request.schema = schema.trim() || "public";
        request.sslMode = sslMode;
      } else if (driver === "mysql") {
        request.charset = charset;
      }
    }
    return request;
  };

  const canSave = (): boolean => {
    if (!name.trim()) return false;
    if (driver === "sqlite") return path.trim().length > 0;
    return host.trim().length > 0 && databaseName.trim().length > 0;
  };

  const choosePath = async () => {
    try {
      const chosen = createNew ? await desktop.chooseDatabaseCreateFile() : await desktop.chooseDatabaseFile();
      if (chosen) setPath(chosen);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("databases.chooseFailed"));
    }
  };

  const testConnection = async () => {
    setTesting(true);
    setTestResult(undefined);
    try {
      const status = await desktop.testDatabase(buildRequest());
      if (status === "connected") {
        setTestResult({ ok: true, message: t("databases.connectionOk") });
      } else {
        setTestResult({ ok: false, message: statusLabel(status, t) });
      }
    } catch (reason) {
      setTestResult({
        ok: false,
        message: reason instanceof Error ? reason.message : t("databases.connectionFailed"),
      });
    } finally {
      setTesting(false);
    }
  };

  const save = async () => {
    setSaving(true);
    try {
      const request = buildRequest();
      // createDatabase creates a new SQLite file; registerDatabase attaches to
      // an existing resource. PostgreSQL and MySQL never create a local file,
      // so they always register.
      const shouldCreate = !isEdit && driver === "sqlite" && createNew;
      if (isEdit) await desktop.updateDatabase(request);
      else if (shouldCreate) await desktop.createDatabase(request);
      else await desktop.registerDatabase(request);
      await onSaved();
      onClose();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("databases.saveFailed"));
    } finally {
      setSaving(false);
    }
  };

  const driverOptions = [
    { value: "sqlite", label: t("databases.driverSqlite") },
    { value: "postgres", label: t("databases.driverPostgres") },
    { value: "mysql", label: t("databases.driverMysql") },
  ];

  const sslOptions = [
    { value: "disable", label: "disable" },
    { value: "prefer", label: "prefer" },
    { value: "require", label: "require" },
    { value: "verify-ca", label: "verify-ca" },
    { value: "verify-full", label: "verify-full" },
  ];

  return (
    <Dialog
      open
      title={isEdit ? t("databases.editTitle") : t("databases.createTitle")}
      description={isEdit ? t("databases.editDescription") : t("databases.createDescription")}
      onOpenChange={(open) => {
        if (!open && !saving) onClose();
      }}
      className="max-w-2xl"
    >
      <div className="muted-scroll space-y-4 overflow-y-auto px-5 py-4">
        <label className="block">
          <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.driver")}</span>
          <Select
            value={driver}
            onValueChange={onDriverChange}
            options={driverOptions}
            ariaLabel={t("databases.driver")}
            disabled={isEdit}
          />
          {isEdit ? <span className="mt-1 block text-xs text-zinc-500">{t("databases.driver")}: {driverBadge(driver, t).label}</span> : null}
        </label>

        <label className="block">
          <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.name")}</span>
          <Input
            autoFocus
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder={t("databases.namePlaceholder")}
          />
        </label>

        {driver === "sqlite" ? (
          <div className="space-y-3">
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.path")}</span>
              <div className="flex gap-2">
                <Input
                  value={path}
                  onChange={(event) => setPath(event.target.value)}
                  placeholder={t("databases.pathPlaceholder")}
                />
                <Button type="button" variant="outline" onClick={() => void choosePath()} aria-label={t("databases.chooseFile")}>
                  <FolderOpen className="size-4" />
                </Button>
              </div>
            </label>
            {!isEdit ? (
              <label className="flex cursor-pointer select-none items-center gap-2 text-xs text-zinc-300">
                <input
                  type="checkbox"
                  className="size-3.5 rounded border-zinc-600 bg-zinc-900 accent-zinc-300"
                  checked={createNew}
                  onChange={(event) => setCreateNew(event.target.checked)}
                />
                {t("databases.createNewFile")}
              </label>
            ) : null}
          </div>
        ) : null}

        {driver === "postgres" || driver === "mysql" ? (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.host")}</span>
                <Input value={host} onChange={(event) => setHost(event.target.value)} placeholder="localhost" />
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.port")}</span>
                <Input
                  type="number"
                  value={port}
                  onChange={(event) => setPort(event.target.value)}
                  placeholder={String(defaultPort(driver))}
                />
              </label>
            </div>
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.database")}</span>
              <Input value={databaseName} onChange={(event) => setDatabaseName(event.target.value)} />
            </label>
            <div className="grid grid-cols-2 gap-3">
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.username")}</span>
                <Input value={username} onChange={(event) => setUsername(event.target.value)} />
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.password")}</span>
                <Input
                  type="password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder={isEdit ? t("databases.passwordPlaceholder") : undefined}
                  autoComplete="new-password"
                />
              </label>
            </div>
            {driver === "postgres" ? (
              <div className="grid grid-cols-2 gap-3">
                <label className="block">
                  <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.schema")}</span>
                  <Input value={schema} onChange={(event) => setSchema(event.target.value)} placeholder="public" />
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.sslMode")}</span>
                  <Select
                    value={sslMode}
                    onValueChange={setSslMode}
                    options={sslOptions}
                    ariaLabel={t("databases.sslMode")}
                  />
                </label>
              </div>
            ) : null}
            {driver === "mysql" ? (
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-zinc-400">{t("databases.charset")}</span>
                <Input value={charset} readOnly aria-readonly className="opacity-70" />
              </label>
            ) : null}
          </div>
        ) : null}
      </div>

      <div className="space-y-3 border-t border-zinc-800 px-5 py-4">
        {testResult ? (
          <div
            className={cn(
              "flex items-center gap-2 rounded-md border px-3 py-2 text-xs",
              testResult.ok
                ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-300"
                : "border-red-500/30 bg-red-500/10 text-red-300",
            )}
          >
            <span>{testResult.ok ? "\u2713" : "\u2717"}</span>
            <span className="min-w-0 flex-1 break-words">{testResult.message}</span>
          </div>
        ) : null}
        <div className="flex items-center justify-between gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => void testConnection()}
            disabled={testing || saving || !canSave()}
          >
            {testing ? <Loader2 className="size-4 animate-spin" /> : <Plug className="size-4" />}
            {t("databases.testConnection")}
          </Button>
          <div className="flex gap-2">
            <Button variant="ghost" onClick={onClose} disabled={saving}>
              {t("common.cancel")}
            </Button>
            <Button onClick={() => void save()} disabled={pending || saving || testing || !canSave()}>
              {saving ? <Loader2 className="size-4 animate-spin" /> : null}
              {isEdit ? t("common.save") : t("common.create")}
            </Button>
          </div>
        </div>
      </div>
    </Dialog>
  );
}

export function DatabasesView() {
  const { t } = useTranslation();
  const setError = useUIStore((state) => state.setError);
  const confirm = useConfirmationStore((state) => state.ask);
  const [items, setItems] = useState<Database[]>([]);
  const [loading, setLoading] = useState(true);
  const [mode, setMode] = useState<DatabaseMode>();
  const [editing, setEditing] = useState<Database>();
  const [menu, setMenu] = useState<{ item: Database; position: ContextMenuPosition }>();

  const load = async () => {
    setLoading(true);
    try {
      setItems(await desktop.listDatabases());
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("databases.loadFailed"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const unregister = async (item: Database) => {
    setMenu(undefined);
    if (
      !(await confirm({
        title: t("databases.unregisterTitle"),
        description: t("databases.unregisterDescription", { name: item.name }),
        confirmLabel: t("databases.unregister"),
      }))
    )
      return;
    try {
      await desktop.deleteDatabase(item.id);
      await load();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("databases.unregisterFailed"));
    }
  };

  const openEdit = (item: Database) => {
    setEditing(item);
    setMode("edit");
    setMenu(undefined);
  };

  return (
    <section className="flex h-full min-h-0 flex-col">
      <PageHeader
        title={t("databases.title")}
        description={t("databases.description")}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => void load()} disabled={loading}>
              {loading ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              {t("common.refresh")}
            </Button>
            <Button onClick={() => setMode("create")}>
              <Plus className="size-4" />
              {t("databases.addDatabase")}
            </Button>
          </div>
        }
      />
      <div className="muted-scroll min-h-0 flex-1 overflow-y-auto p-8">
        {!loading && items.length === 0 ? (
          <EmptyState
            icon={DatabaseIcon}
            title={t("databases.emptyTitle")}
            description={t("databases.emptyDescription")}
            action={{ label: t("databases.addDatabase"), onClick: () => setMode("create") }}
          />
        ) : (
          <div className="overflow-hidden rounded-xl border border-zinc-800">
            {items.map((item) => {
              const badge = driverBadge(item.driver, t);
              const detail = driverDetail(item);
              return (
                <div
                  key={item.id}
                  className="group grid grid-cols-[minmax(180px,1fr)_120px_minmax(220px,2fr)_140px_36px] items-center gap-4 border-b border-zinc-800 px-4 py-3.5 last:border-0 hover:bg-zinc-900"
                  onContextMenu={(event) => {
                    event.preventDefault();
                    setMenu({ item, position: contextMenuPosition(event, { width: 180, height: 80 }) });
                  }}
                >
                  <button
                    type="button"
                    className="flex min-w-0 items-center gap-3 rounded text-left outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"
                    onClick={() => openEdit(item)}
                    onKeyDown={(event) => {
                      if (event.key === "ContextMenu" || (event.shiftKey && event.key === "F10")) {
                        event.preventDefault();
                        setMenu({
                          item,
                          position: contextMenuPosition(contextMenuPointFromElement(event.currentTarget), {
                            width: 180,
                            height: 80,
                          }),
                        });
                      }
                    }}
                    title={statusLabel(item.status, t)}
                  >
                    <span
                      className={cn("size-2 shrink-0 rounded-full", statusDotClass(item.status))}
                      aria-hidden
                    />
                    <span className="flex size-8 shrink-0 items-center justify-center rounded-md border border-zinc-800 bg-zinc-900">
                      <DatabaseIcon className="size-4 text-emerald-300" />
                    </span>
                    <span className="truncate text-sm font-medium text-zinc-100">{item.name}</span>
                  </button>
                  <span
                    className={cn(
                      "inline-flex w-fit items-center rounded-full border px-2 py-0.5 text-[11px] font-medium",
                      badge.className,
                    )}
                  >
                    {badge.label}
                  </span>
                  <span className="truncate font-mono text-xs text-zinc-500" title={detail}>
                    {detail}
                  </span>
                  <span className="text-xs text-zinc-500">{formatDate(item.updatedAt)}</span>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="size-7 p-0 opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
                    onClick={(event) =>
                      setMenu({
                        item,
                        position: contextMenuPosition(contextMenuPointFromElement(event.currentTarget), {
                          width: 180,
                          height: 80,
                        }),
                      })
                    }
                    aria-label={t("databases.options", { name: item.name })}
                  >
                    <MoreHorizontal className="size-4" />
                  </Button>
                </div>
              );
            })}
          </div>
        )}
      </div>
      {menu ? (
        <ContextMenu
          position={menu.position}
          ariaLabel={t("databases.options", { name: menu.item.name })}
          className="w-44"
          onClose={() => setMenu(undefined)}
        >
          <button
            type="button"
            role="menuitem"
            onClick={() => openEdit(menu.item)}
            className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-xs text-zinc-200 hover:bg-zinc-800 focus-visible:bg-zinc-800"
          >
            <Pencil className="size-3.5" />
            {t("common.edit")}
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => void unregister(menu.item)}
            className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-xs text-red-300 hover:bg-red-500/10 focus-visible:bg-red-500/10"
          >
            <Trash2 className="size-3.5" />
            {t("databases.unregister")}
          </button>
        </ContextMenu>
      ) : null}
      {mode ? (
        <DatabaseDialog
          key={`${mode}-${editing?.id ?? "new"}`}
          mode={mode}
          item={mode === "edit" ? editing : undefined}
          pending={loading}
          onClose={() => {
            setMode(undefined);
            setEditing(undefined);
          }}
          onSaved={load}
        />
      ) : null}
    </section>
  );
}
