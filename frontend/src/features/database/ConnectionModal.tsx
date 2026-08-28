import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { desktop } from "@/lib/bridge";
import type { Database, DatabaseDriver, SaveDatabaseRequest } from "@/lib/types";
import { Modal, ModalActions } from "../../components/primitives/Modal";
import { Field, TextInput } from "../../components/primitives/Field";
import { SegmentedControl } from "../../components/primitives/SegmentedControl";
import { Icon } from "../../components/icons";
import { Tooltip } from "../../components/Tooltip";
import { cn } from "../../utils/cn";
import {
  DB_ENGINES,
  engineById,
  fileField,
  postgresExtras,
  redisExtras,
  serverFields,
  sugardbFields,
  type ConnMode,
  type EngineField,
} from "./db-engines";

type TestState = "idle" | "testing" | "ok" | "fail";

/**
 * Create / edit a database connection against the real Desktop bridge.
 * Engines render themselves from `db-engines.ts`; the test probe calls
 * `testDatabase`, and saving routes to create/register/update per driver.
 */
export function ConnectionModal({
  existing = null,
  onClose,
  onSaved,
}: {
  existing?: Database | null;
  onClose: () => void;
  /** called after a successful create/register/update so the list reloads */
  onSaved: () => void;
}) {
  const { t } = useTranslation();
  const [driver, setDriver] = useState<DatabaseDriver>(existing?.driver ?? "sqlite");
  const [name, setName] = useState(existing?.name ?? "");
  const [values, setValues] = useState<Record<string, string>>(() => ({
    path: existing?.path ?? "",
    host: existing?.host ?? "",
    port: existing?.port ? String(existing.port) : "",
    database: existing?.database ?? "",
    username: existing?.username ?? "",
    password: "",
    schema: existing?.schema ?? "public",
    sslmode: existing?.sslMode ?? "prefer",
    dbIndex: existing?.dbIndex !== undefined ? String(existing.dbIndex) : "0",
    address: existing?.address ?? "",
    clientName: existing?.clientName ?? "",
  }));
  const [useTLS, setUseTLS] = useState(Boolean(existing?.useTLS));
  const [createNewFile, setCreateNewFile] = useState(false);
  const [test, setTest] = useState<TestState>("idle");
  const [saving, setSaving] = useState(false);

  const engine = engineById(driver);
  const mode: ConnMode =
    driver === "sqlite" || driver === "duckdb" ? "file" : driver === "sugardb" ? "embedded" : "server";

  const set = (k: string, v: string) => {
    setValues((prev) => ({ ...prev, [k]: v }));
    setTest("idle");
  };

  const selectEngine = (id: DatabaseDriver) => {
    setDriver(id);
    setTest("idle");
  };

  const fields: EngineField[] = useMemo(() => {
    if (mode === "file") return [fileField()];
    if (mode === "embedded") return sugardbFields();
    const shared = serverFields(engine)
      // Redis has no named database; its logical index is dbIndex below.
      .filter((f) => !(driver === "redis" && f.key === "database"));
    const extras = driver === "postgres" ? postgresExtras() : driver === "redis" ? redisExtras() : [];
    return [...shared, ...extras].filter((f) => !f.mode || f.mode === mode);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [engine, mode, driver]);

  const requiredFilled = fields.every((f) => f.optional || (values[f.key] ?? f.default ?? "").trim().length > 0);
  const canSubmit = name.trim().length > 0 && requiredFilled && !saving;

  const buildRequest = (): SaveDatabaseRequest => {
    const req: SaveDatabaseRequest = {
      ...(existing ? { id: existing.id } : {}),
      name: name.trim(),
      driver,
    };
    // blank keeps the previously stored secret; a typed value rotates it
    const applyPassword = () => {
      if (values.password) req.password = values.password;
      else if (existing?.passwordRef) req.passwordRef = existing.passwordRef;
    };
    if (driver === "sugardb") {
      req.path = values.path ?? "";
      req.dbIndex = values.dbIndex ? Number(values.dbIndex) : 0;
      applyPassword();
    } else if (driver === "sqlite" || driver === "duckdb") {
      req.path = values.path ?? "";
    } else {
      req.host = values.host ?? "";
      req.port = values.port ? Number(values.port) : undefined;
      req.username = values.username ?? "";
      applyPassword();
      if (driver === "postgres") {
        req.database = values.database ?? "";
        req.schema = values.schema || "public";
        req.sslMode = values.sslmode || "prefer";
      }
      if (driver === "mysql") {
        req.database = values.database ?? "";
      }
      if (driver === "redis") {
        req.dbIndex = values.dbIndex ? Number(values.dbIndex) : 0;
        req.useTLS = useTLS;
        req.address = values.address ?? "";
        req.clientName = values.clientName ?? "";
      }
    }
    return req;
  };

  const runTest = async () => {
    if (!requiredFilled || test === "testing") return;
    setTest("testing");
    try {
      const status = await desktop.testDatabase(buildRequest());
      setTest(status === "connected" ? "ok" : "fail");
    } catch {
      setTest("fail");
    }
  };

  const pickFile = async () => {
    try {
      const file = createNewFile
        ? await desktop.chooseDatabaseCreateFile()
        : await desktop.chooseDatabaseFile();
      if (file) set("path", file);
    } catch {
      /* picker canceled */
    }
  };

  const pickDirectory = async () => {
    try {
      const dir = await desktop.chooseDirectory(t("databases.chooseDirectory"));
      if (dir) set("path", dir);
    } catch {
      /* picker canceled */
    }
  };

  const submit = async () => {
    if (!canSubmit) return;
    setSaving(true);
    try {
      const req = buildRequest();
      if (existing) await desktop.updateDatabase(req);
      else if ((driver === "sqlite" || driver === "duckdb") && createNewFile) await desktop.createDatabase(req);
      else await desktop.registerDatabase(req);
      onSaved();
    } catch {
      setSaving(false);
    }
  };

  const preview = useMemo(() => {
    if (driver === "sqlite" || driver === "duckdb") return `${driver}://${values.path || ":"}`;
    if (driver === "sugardb") {
      const db = values.dbIndex || "0";
      return `sugardb://${values.path || "(app data)"}/${db}`;
    }
    if (driver === "redis") {
      if (values.address) return values.address;
      const auth = values.username ? `${values.username}@` : "";
      const host = values.host || "localhost";
      const port = values.port || engine.defaultPort || "";
      const db = values.dbIndex || "0";
      const tls = useTLS ? "?ssl=true" : "";
      return `redis://${auth}${host}:${port}/${db}${tls}`;
    }
    const auth = values.username || "user";
    const host = values.host || "localhost";
    const port = values.port || engine.defaultPort || "";
    const db = values.database || "database";
    const extra = driver === "postgres" ? `?sslmode=${values.sslmode || "prefer"}` : "";
    return `${driver}://${auth}@${host}:${port}/${db}${extra}`;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [driver, values, useTLS]);

  return (
    <Modal
      title={existing ? t("databases.editTitle") : t("databases.createTitle")}
      icon="Database"
      size="md"
      onClose={onClose}
      bodyClassName="min-h-0 flex-1 overflow-y-auto"
      footer={
        <>
          <TestIndicator state={test} onTest={() => void runTest()} disabled={!requiredFilled} />
          <ModalActions
            onCancel={onClose}
            onConfirm={() => void submit()}
            confirmLabel={t("dbnew.createConnection")}
            disabled={!canSubmit}
          />
        </>
      }
    >
      <div className="space-y-5 p-4">
        {/* engine picker */}
        <section>
          <p className="mb-2 text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">{t("dbnew.engine")}</p>
          <div className={cn("grid gap-2", existing ? "grid-cols-1" : "grid-cols-3")}>
            {DB_ENGINES.map((e) => {
              const active = e.id === driver;
              return (
                <button
                  key={e.id}
                  onClick={() => selectEngine(e.id)}
                  disabled={Boolean(existing)}
                  title={undefined}
                  className={cn(
                    "flex items-start gap-2.5 rounded-lg border p-2.5 text-left transition",
                    active
                      ? "border-ink-400 bg-ink-800/70"
                      : "border-ink-700/80 bg-ink-850/40 hover:border-ink-600 hover:bg-ink-850",
                    existing && e.id !== driver && "hidden",
                    existing && "cursor-default",
                  )}
                >
                  <span
                    className={cn(
                      "grid h-8 w-8 shrink-0 place-items-center rounded-lg border",
                      active ? "border-ink-500 bg-ink-750 text-fg" : "border-ink-700 bg-ink-900 text-fg-subtle",
                    )}
                  >
                    <Icon name={e.icon} className="h-4 w-4" />
                  </span>
                  <span className="min-w-0">
                    <span className="flex items-center gap-1.5">
                      <span className="text-[12.5px] font-medium text-fg">{e.name}</span>
                      {active && <Icon name="Check" className="h-3.5 w-3.5 text-success-fg" />}
                    </span>
                    <span className="mt-0.5 line-clamp-2 block text-[10.5px] leading-snug text-fg-faint">
                      {t(e.blurbKey)}
                    </span>
                  </span>
                </button>
              );
            })}
          </div>
        </section>

        {/* connection details */}
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">{t("dbnew.connection")}</p>
            {!existing && (
              <SegmentedControl
                value={mode}
                onChange={(m) => {
                  void m;
                  /* driver determines the mode; kept for layout parity */
                }}
                segments={[
                  {
                    value: mode,
                    label: t(mode === "file" ? "dbnew.fileMode" : mode === "embedded" ? "dbnew.embeddedMode" : "dbnew.serverMode"),
                  },
                ]}
              />
            )}
          </div>

          <Field label={t("dbnew.displayName")} required>
            <TextInput value={name} onChange={setName} placeholder={t("dbnew.namePlaceholder")} autoFocus mono={!existing} />
          </Field>

          {mode === "file" ? (
            <>
              {!existing && (
                <label className="flex items-center gap-2 text-[12px] text-fg-subtle">
                  <input
                    type="checkbox"
                    checked={createNewFile}
                    onChange={(e) => {
                      setCreateNewFile(e.target.checked);
                      setTest("idle");
                    }}
                  />
                  {t("databases.createNewFile")}
                </label>
              )}
              <Field label={t("databases.path")} required>
                <div className="flex gap-2">
                  <TextInput value={values.path ?? ""} onChange={(v) => set("path", v)} placeholder={t("databases.pathPlaceholder")} mono />
                  <button
                    onClick={() => void pickFile()}
                    className="flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[11.5px] text-fg-muted transition hover:bg-ink-750"
                  >
                    <Icon name="HardDrive" className="h-3.5 w-3.5" />
                    {t("databases.chooseFile")}
                  </button>
                </div>
              </Field>
            </>
          ) : mode === "embedded" ? (
            <>
              <Field label={t("databases.dataDir")} hint={t("databases.dataDirHint")}>
                <div className="flex gap-2">
                  <TextInput
                    value={values.path ?? ""}
                    onChange={(v) => set("path", v)}
                    placeholder={t("databases.dataDirPlaceholder")}
                    mono
                  />
                  <button
                    onClick={() => void pickDirectory()}
                    className="flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[11.5px] text-fg-muted transition hover:bg-ink-750"
                  >
                    <Icon name="HardDrive" className="h-3.5 w-3.5" />
                    {t("databases.chooseDirectory")}
                  </button>
                </div>
              </Field>
              <div className="grid grid-cols-2 gap-3">
                {fields.map((f) => (
                  <Field key={f.key} label={t(f.labelKey)} required={!f.optional}>
                    {f.key === "password" ? (
                      <input
                        type="password"
                        autoComplete="new-password"
                        value={values[f.key] ?? ""}
                        placeholder={existing?.passwordRef ? t("databases.passwordPlaceholder") : undefined}
                        onChange={(e) => set(f.key, e.target.value)}
                        className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
                      />
                    ) : (
                      <TextInput
                        value={values[f.key] ?? f.default ?? ""}
                        onChange={(v) => set(f.key, v)}
                        type={f.type === "number" ? "number" : "text"}
                      />
                    )}
                  </Field>
                ))}
              </div>
            </>
          ) : (
            <div className="grid grid-cols-2 gap-3">
              {fields.map((f) => (
                <Field
                  key={f.key}
                  label={t(f.labelKey)}
                  required={!f.optional}
                  className={f.key === "address" ? "col-span-2" : undefined}
                >
                  {f.key === "password" ? (
                    <input
                      type="password"
                      autoComplete="new-password"
                      value={values[f.key] ?? ""}
                      placeholder={existing?.passwordRef ? t("databases.passwordPlaceholder") : undefined}
                      onChange={(e) => set(f.key, e.target.value)}
                      className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
                    />
                  ) : f.key === "sslmode" ? (
                    <select
                      value={values[f.key] ?? f.default ?? "prefer"}
                      onChange={(e) => set(f.key, e.target.value)}
                      className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2 text-[12.5px] capitalize text-fg focus:border-ink-400 focus:outline-none"
                    >
                      {["disable", "prefer", "require", "verify-ca", "verify-full"].map((m) => (
                        <option key={m} value={m}>
                          {m}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <TextInput
                      value={values[f.key] ?? f.default ?? ""}
                      onChange={(v) => set(f.key, v)}
                      placeholder={f.placeholderKey ? t(f.placeholderKey) : undefined}
                      type={f.type === "number" ? "number" : "text"}
                      mono={f.key === "address"}
                    />
                  )}
                </Field>
              ))}
              {driver === "redis" && (
                <label className="col-span-2 flex items-center gap-2 text-[12px] text-fg-subtle">
                  <input
                    type="checkbox"
                    checked={useTLS}
                    onChange={(e) => {
                      setUseTLS(e.target.checked);
                      setTest("idle");
                    }}
                  />
                  {t("databases.useTls")}
                </label>
              )}
            </div>
          )}
        </section>

        {/* connection string preview */}
        <section>
          <p className="mb-1.5 flex items-center gap-1.5 text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">
            {t("dbnew.preview")}
            <Tooltip content={t("dbnew.previewHint")} side="top">
              <Icon name="Info" className="h-3 w-3 cursor-help text-fg-faint" />
            </Tooltip>
          </p>
          <div className="flex items-center gap-2 rounded-lg border border-ink-700/80 bg-ink-950/60 px-3 py-2">
            <Icon name="Cable" className="h-3.5 w-3.5 shrink-0 text-fg-faint" />
            <code className="min-w-0 flex-1 truncate font-mono text-[11px] text-fg-subtle">{preview}</code>
          </div>
        </section>
      </div>
    </Modal>
  );
}

/** Test-connection button that reflects its live probe state. */
function TestIndicator({
  state,
  onTest,
  disabled,
}: {
  state: TestState;
  onTest: () => void;
  disabled: boolean;
}) {
  const { t } = useTranslation();
  const map = {
    idle: { icon: "Zap", text: t("databases.testConnection"), cls: "text-fg-muted hover:bg-ink-750" },
    testing: { icon: "Loader2", text: t("dbnew.testTesting"), cls: "text-fg-subtle" },
    ok: { icon: "Check", text: t("databases.connectionOk"), cls: "border-success/30 bg-success/10 text-success-fg" },
    fail: { icon: "AlertTriangle", text: t("dbnew.testFailed"), cls: "border-danger/30 bg-danger/10 text-danger-fg" },
  }[state];

  return (
    <Tooltip content={state === "testing" ? t("dbnew.testTesting") : map.text} side="top">
      <button
        onClick={onTest}
        disabled={disabled || state === "testing"}
        aria-label={map.text}
        className={cn(
          "flex h-7 items-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[11.5px] transition disabled:cursor-not-allowed disabled:opacity-50",
          map.cls,
        )}
      >
        <Icon name={map.icon} className={cn("h-3.5 w-3.5", state === "testing" && "animate-spin")} />
        {map.text}
      </button>
    </Tooltip>
  );
}
