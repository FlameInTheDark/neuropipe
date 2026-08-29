import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { desktop } from "@/lib/bridge";
import type { SaveStorageRequest, Storage, StorageDriver, StorageTLSMode } from "@/lib/types";
import { Modal, ModalActions } from "../../components/primitives/Modal";
import { Field, TextInput } from "../../components/primitives/Field";
import { Icon } from "../../components/icons";
import { Tooltip } from "../../components/Tooltip";
import { cn } from "../../utils/cn";

type TestState = "idle" | "testing" | "ok" | "fail";

/** Engine cards for the picker — copy comes from i18n. */
const ENGINES: { id: StorageDriver; name: string; icon: string; blurbKey: string; defaultPort?: number }[] = [
  { id: "s3", name: "S3 compatible", icon: "Cloud", blurbKey: "storages.blurbS3" },
  { id: "ftp", name: "FTP / FTPS", icon: "Globe", blurbKey: "storages.blurbFtp", defaultPort: 21 },
];

const TLS_MODES: { value: StorageTLSMode; labelKey: string }[] = [
  { value: "", labelKey: "storages.tlsAuto" },
  { value: "none", labelKey: "storages.tlsNone" },
  { value: "explicit", labelKey: "storages.tlsExplicit" },
  { value: "implicit", labelKey: "storages.tlsImplicit" },
];

/**
 * Create / edit an S3 or FTP storage connection against the real Desktop
 * bridge. The test probe calls `testStorage` with the unsaved form values;
 * saving routes to registerStorage / updateStorage. Secrets are write-only:
 * a blank field keeps the previously stored value.
 */
export function StorageConnectionModal({
  existing = null,
  onClose,
  onSaved,
}: {
  existing?: Storage | null;
  onClose: () => void;
  /** called after a successful register/update so the list reloads */
  onSaved: () => void;
}) {
  const { t } = useTranslation();
  const [driver, setDriver] = useState<StorageDriver>(existing?.driver ?? "s3");
  const [name, setName] = useState(existing?.name ?? "");
  const [endpoint, setEndpoint] = useState(existing?.endpoint ?? "");
  const [region, setRegion] = useState(existing?.region ?? "");
  const [bucket, setBucket] = useState(existing?.bucket ?? "");
  const [accessKey, setAccessKey] = useState(existing?.accessKey ?? "");
  const [secret, setSecret] = useState("");
  const [secure, setSecure] = useState(existing?.secure === undefined ? true : existing.secure);
  const [host, setHost] = useState(existing?.host ?? "");
  const [port, setPort] = useState(existing?.port ? String(existing.port) : "");
  const [username, setUsername] = useState(existing?.username ?? "");
  const [password, setPassword] = useState("");
  const [tlsMode, setTlsMode] = useState<StorageTLSMode>(existing?.tlsMode ?? "");
  const [baseDir, setBaseDir] = useState(existing?.baseDir ?? "");
  const [publicBaseUrl, setPublicBaseUrl] = useState(existing?.publicBaseUrl ?? "");
  const [test, setTest] = useState<TestState>("idle");
  const [saving, setSaving] = useState(false);

  const touch = () => setTest("idle");

  const buildRequest = (): SaveStorageRequest => {
    const req: SaveStorageRequest = {
      ...(existing ? { id: existing.id } : {}),
      name: name.trim(),
      driver,
    };
    if (driver === "s3") {
      req.endpoint = endpoint.trim();
      req.region = region.trim();
      req.bucket = bucket.trim();
      req.accessKey = accessKey.trim();
      req.secure = secure;
      // blank keeps the previously stored secret; typed value rotates it
      if (secret) req.secret = secret;
      else if (existing?.secretRef) req.secretRef = existing.secretRef;
    } else {
      req.host = host.trim();
      req.port = port ? Number(port) : undefined;
      req.username = username.trim();
      req.tlsMode = tlsMode;
      req.baseDir = baseDir.trim();
      if (password) req.password = password;
      else if (existing?.passwordRef) req.passwordRef = existing.passwordRef;
    }
    req.publicBaseUrl = publicBaseUrl.trim();
    return req;
  };

  const requiredFilled =
    name.trim().length > 0 &&
    (driver === "s3"
      ? endpoint.trim().length > 0 && bucket.trim().length > 0
      : host.trim().length > 0);
  const canSubmit = requiredFilled && !saving;

  const runTest = async () => {
    if (!requiredFilled || test === "testing") return;
    setTest("testing");
    try {
      const status = await desktop.testStorage(buildRequest());
      setTest(status === "connected" ? "ok" : "fail");
    } catch {
      setTest("fail");
    }
  };

  const submit = async () => {
    if (!canSubmit) return;
    setSaving(true);
    try {
      const req = buildRequest();
      if (existing) await desktop.updateStorage(req);
      else await desktop.registerStorage(req);
      onSaved();
    } catch {
      setSaving(false);
    }
  };

  const preview = useMemo(() => {
    if (driver === "s3") {
      const scheme = secure ? "https" : "http";
      const ep = endpoint || "s3.amazonaws.com";
      const rg = region ? ` (${region})` : "";
      const key = accessKey ? `${accessKey}@` : "";
      return `${scheme}://${key}${ep}/${bucket || "bucket"}${rg}`;
    }
    const scheme = tlsMode === "implicit" ? "ftps" : tlsMode === "explicit" ? "ftp+tls" : "ftp";
    const auth = username ? `${username}@` : "";
    const p = port || "21";
    const dir = baseDir ? `/${baseDir.replace(/^\/+|\/+$/g, "")}` : "";
    return `${scheme}://${auth}${host || "host"}:${p}${dir}`;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [driver, endpoint, region, bucket, accessKey, secure, host, port, username, tlsMode, baseDir]);

  return (
    <Modal
      title={existing ? t("storages.editTitle") : t("storages.createTitle")}
      icon="Cloud"
      size="md"
      onClose={onClose}
      bodyClassName="min-h-0 flex-1 overflow-y-auto"
      footer={
        <>
          <TestIndicator state={test} onTest={() => void runTest()} disabled={!requiredFilled} />
          <ModalActions
            onCancel={onClose}
            onConfirm={() => void submit()}
            confirmLabel={t("storages.createConnection")}
            disabled={!canSubmit}
          />
        </>
      }
    >
      <div className="space-y-5 p-4">
        {/* engine picker */}
        <section>
          <p className="mb-2 text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">{t("storages.engine")}</p>
          <div className={cn("grid gap-2", existing ? "grid-cols-1" : "grid-cols-2")}>
            {ENGINES.map((e) => {
              const active = e.id === driver;
              return (
                <button
                  key={e.id}
                  onClick={() => {
                    setDriver(e.id);
                    touch();
                  }}
                  disabled={Boolean(existing)}
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
          <p className="text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">{t("storages.connection")}</p>

          <Field label={t("storages.displayName")} required>
            <TextInput value={name} onChange={(v) => { setName(v); touch(); }} placeholder={t("storages.namePlaceholder")} autoFocus mono={!existing} />
          </Field>

          {driver === "s3" ? (
            <>
              <div className="grid grid-cols-2 gap-3">
                <Field label={t("storages.endpoint")} required hint={t("storages.endpointHint")}>
                  <TextInput value={endpoint} onChange={(v) => { setEndpoint(v); touch(); }} placeholder={t("storages.endpointPlaceholder")} mono />
                </Field>
                <Field label={t("storages.region")}>
                  <TextInput value={region} onChange={(v) => { setRegion(v); touch(); }} placeholder={t("storages.regionPlaceholder")} />
                </Field>
                <Field label={t("storages.bucket")} required>
                  <TextInput value={bucket} onChange={(v) => { setBucket(v); touch(); }} placeholder={t("storages.bucketPlaceholder")} mono />
                </Field>
                <Field label={t("storages.accessKey")}>
                  <TextInput value={accessKey} onChange={(v) => { setAccessKey(v); touch(); }} placeholder={t("storages.accessKeyPlaceholder")} mono />
                </Field>
              </div>
              <Field label={t("storages.secretKey")} hint={existing?.secretRef ? t("storages.secretRotateHint") : t("storages.secretHint")}>
                <input
                  type="password"
                  autoComplete="new-password"
                  value={secret}
                  onChange={(e) => { setSecret(e.target.value); touch(); }}
                  className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
                />
              </Field>
              <label className="flex items-center gap-2 text-[12px] text-fg-subtle">
                <input
                  type="checkbox"
                  checked={secure}
                  onChange={(e) => { setSecure(e.target.checked); touch(); }}
                />
                {t("storages.useHttps")}
              </label>
            </>
          ) : (
            <>
              <div className="grid grid-cols-[1fr_100px] gap-3">
                <Field label={t("storages.host")} required>
                  <TextInput value={host} onChange={(v) => { setHost(v); touch(); }} placeholder={t("storages.hostPlaceholder")} mono />
                </Field>
                <Field label={t("storages.port")}>
                  <TextInput value={port} onChange={(v) => { setPort(v); touch(); }} type="number" placeholder="21" />
                </Field>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <Field label={t("storages.username")}>
                  <TextInput value={username} onChange={(v) => { setUsername(v); touch(); }} placeholder={t("storages.usernamePlaceholder")} />
                </Field>
                <Field label={t("storages.tlsMode")}>
                  <select
                    value={tlsMode}
                    onChange={(e) => { setTlsMode(e.target.value as StorageTLSMode); touch(); }}
                    className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2 text-[12.5px] text-fg focus:border-ink-400 focus:outline-none"
                  >
                    {TLS_MODES.map((m) => (
                      <option key={m.value} value={m.value}>{t(m.labelKey)}</option>
                    ))}
                  </select>
                </Field>
              </div>
              <Field label={t("storages.password")} hint={existing?.passwordRef ? t("storages.secretRotateHint") : undefined}>
                <input
                  type="password"
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => { setPassword(e.target.value); touch(); }}
                  className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
                />
              </Field>
              <Field label={t("storages.baseDir")} hint={t("storages.baseDirHint")}>
                <TextInput value={baseDir} onChange={(v) => { setBaseDir(v); touch(); }} placeholder={t("storages.baseDirPlaceholder")} mono />
              </Field>
            </>
          )}

          {/* optional public URL base shared by both engines */}
          <Field label={t("storages.publicBaseUrl")} hint={t("storages.publicBaseUrlHint")}>
            <TextInput value={publicBaseUrl} onChange={(v) => { setPublicBaseUrl(v); touch(); }} placeholder={t("storages.publicBaseUrlPlaceholder")} mono />
          </Field>
        </section>

        {/* connection string preview */}
        <section>
          <p className="mb-1.5 flex items-center gap-1.5 text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">
            {t("storages.preview")}
            <Tooltip content={t("storages.previewHint")} side="top">
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
    idle: { icon: "Zap", text: t("storages.testConnection"), cls: "text-fg-muted hover:bg-ink-750" },
    testing: { icon: "Loader2", text: t("storages.testTesting"), cls: "text-fg-subtle" },
    ok: { icon: "Check", text: t("storages.connectionOk"), cls: "border-success/30 bg-success/10 text-success-fg" },
    fail: { icon: "AlertTriangle", text: t("storages.testFailed"), cls: "border-danger/30 bg-danger/10 text-danger-fg" },
  }[state];

  return (
    <Tooltip content={state === "testing" ? t("storages.testTesting") : map.text} side="top">
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
