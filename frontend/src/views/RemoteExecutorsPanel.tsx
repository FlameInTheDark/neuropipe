import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import { desktop } from "@/lib/bridge";
import type {
  ExecutorCreateResult,
  ExecutorLLMMode,
  RemoteExecutorConfig,
  RemoteExecutorListItem,
  RemoteExecutorProvider,
  RemoteExecutorStatus,
} from "@/lib/types";
import type { Workspace } from "@/features/workspace/useWorkspace";
import { ask } from "@/stores/confirmation";
import { SectionCard } from "./SettingsView";
import { Button, Toggle } from "../components/ui";
import { Modal } from "../components/primitives/Modal";
import { Field, TextInput } from "../components/primitives/Field";
import { Dropdown } from "../components/Dropdown";
import { EmptyState } from "../components/ViewShell";
import { cn } from "../utils/cn";

const PROVIDER_KINDS = ["openai-compatible", "ollama", "llamacpp"] as const;
type ProviderKind = (typeof PROVIDER_KINDS)[number];

/** Settings panel managing remote executor registrations, connections, and runtime configuration. */
export function RemoteExecutorsPanel({ workspace }: { workspace: Workspace }) {
  const { t } = useTranslation();
  const [executors, setExecutors] = useState<RemoteExecutorListItem[]>([]);
  const [addOpen, setAddOpen] = useState(false);
  const [configTarget, setConfigTarget] = useState<string | null>(null);
  const [tokenResult, setTokenResult] = useState<ExecutorCreateResult | null>(null);
  const [rotatedToken, setRotatedToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const reload = useCallback(async () => {
    try {
      setExecutors(await desktop.listRemoteExecutors());
    } catch {
      /* transient backend error; keep the current list */
    }
  }, []);

  useEffect(() => {
    void reload();
    const off = Events.On("executor.status.updated", () => void reload());
    return () => off();
  }, [reload]);

  const notifyError = (key: string) => workspace.notify(t(key), "AlertTriangle");

  return (
    <div className="space-y-3">
      <SectionCard
        title={t("executors.title")}
        action={
          <Button
            icon="Plus"
            variant="primary"
            onClick={() => setAddOpen(true)}
          >
            {t("executors.add")}
          </Button>
        }
      >
        <p className="mb-3 text-[12px] leading-relaxed text-fg-subtle">{t("executors.description")}</p>
        {executors.length === 0 ? (
          <EmptyState icon="Server" title={t("executors.emptyTitle")} hint={t("executors.emptyDescription")} />
        ) : (
          <div className="overflow-hidden rounded-lg border border-seam">
            {executors.map((item) => (
              <ExecutorRow
                key={item.id}
                item={item}
                busy={busy}
                onConfigure={() => setConfigTarget(item.id)}
                onTest={async () => {
                  setBusy(true);
                  try {
                    await desktop.getRemoteExecutorStatus(item.id);
                  } catch {
                    notifyError("executors.testFailed");
                  } finally {
                    setBusy(false);
                    void reload();
                  }
                }}
                onRotate={async () => {
                  const ok = await ask({
                    title: t("executors.rotateTitle"),
                    description: t("executors.rotateDescription"),
                    confirmLabel: t("executors.rotate"),
                  });
                  if (!ok) return;
                  setBusy(true);
                  try {
                    const token = await desktop.rotateExecutorToken(item.id);
                    setRotatedToken(token);
                  } catch {
                    notifyError("executors.rotateFailed");
                  } finally {
                    setBusy(false);
                  }
                }}
                onRemove={async () => {
                  const ok = await ask({
                    title: t("executors.removeTitle", { name: item.name }),
                    description: t("executors.removeDescription"),
                    confirmLabel: t("common.delete"),
                    danger: true,
                  });
                  if (!ok) return;
                  setBusy(true);
                  try {
                    await desktop.removeRemoteExecutor(item.id);
                    await workspace.refresh();
                    void reload();
                  } catch {
                    notifyError("executors.removeFailed");
                  } finally {
                    setBusy(false);
                  }
                }}
              />
            ))}
          </div>
        )}
      </SectionCard>

      <AddExecutorDialog
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onCreated={(result) => {
          setAddOpen(false);
          setTokenResult(result);
          void workspace.refresh();
          void reload();
        }}
        onError={() => notifyError("executors.addFailed")}
      />

      <ConfigureExecutorDialog
        executorId={configTarget}
        onClose={() => setConfigTarget(null)}
        onSaved={() => {
          void workspace.refresh();
          void reload();
        }}
        onError={() => notifyError("executors.configSaveFailed")}
      />

      <TokenOnceModal
        title={t("executors.tokenTitle")}
        description={t("executors.tokenDescription")}
        token={tokenResult?.token ?? rotatedToken}
        onClose={() => {
          setTokenResult(null);
          setRotatedToken(null);
        }}
      />
    </div>
  );
}

function ExecutorRow({
  item,
  busy,
  onConfigure,
  onTest,
  onRotate,
  onRemove,
}: {
  item: RemoteExecutorListItem;
  busy: boolean;
  onConfigure: () => void;
  onTest: () => Promise<void>;
  onRotate: () => Promise<void>;
  onRemove: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const status = item.status;
  return (
    <div className="flex items-center gap-3 border-b border-seam/70 px-3 py-2.5 last:border-b-0">
      <span
        className={cn(
          "h-2 w-2 shrink-0 rounded-full",
          status.online ? "bg-success shadow-[0_0_6px_color-mix(in_srgb,var(--status-success)_70%,transparent)]" : "bg-ink-600",
        )}
        aria-hidden
      />
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-2">
          <span className="truncate text-[13px] font-medium text-fg">{item.name}</span>
          <span
            className={cn(
              "rounded-full border px-1.5 py-px text-[10px]",
              item.llmMode === "local"
                ? "border-violet-500/40 bg-violet-500/10 text-violet-300"
                : "border-info/40 bg-info/10 text-info-fg",
            )}
          >
            {t(item.llmMode === "local" ? "executors.modeLocal" : "executors.modeProxy")}
          </span>
        </span>
        <span className="mt-[1px] block truncate font-mono text-[11px] text-fg-faint">
          {item.address}
          {status.online && status.version
            ? ` · ${t("executors.versionShort", { version: status.version })}${status.platform ? ` · ${status.platform}` : ""}`
            : status.message
              ? ` · ${status.message}`
              : ""}
        </span>
      </span>
      <Button variant="ghost" icon="Settings2" disabled={busy} onClick={onConfigure}>
        {t("executors.configure")}
      </Button>
      <Button variant="ghost" icon="RefreshCw" disabled={busy} onClick={() => void onTest()}>
        {t("executors.test")}
      </Button>
      <Button variant="ghost" icon="KeyRound" disabled={busy} onClick={() => void onRotate()}>
        {t("executors.rotate")}
      </Button>
      <Button variant="ghost" icon="Trash2" disabled={busy} onClick={() => void onRemove()}>
        {t("common.delete")}
      </Button>
    </div>
  );
}

function AddExecutorDialog({
  open,
  onClose,
  onCreated,
  onError,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (result: ExecutorCreateResult) => void;
  onError: () => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [token, setToken] = useState("");
  const [useTLS, setUseTLS] = useState(false);
  const [testResult, setTestResult] = useState<RemoteExecutorStatus | null>(null);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);

  const reset = () => {
    setName("");
    setAddress("");
    setToken("");
    setUseTLS(false);
    setTestResult(null);
  };

  const create = async () => {
    setSaving(true);
    try {
      const result = await desktop.addRemoteExecutor({ name, address, token: token || undefined, useTLS });
      reset();
      onCreated(result);
    } catch {
      onError();
    } finally {
      setSaving(false);
    }
  };

  const test = async () => {
    setTesting(true);
    try {
      setTestResult(await desktop.testRemoteExecutor(address, token, useTLS));
    } catch {
      setTestResult({ online: false, activeRuns: 0, maxConcurrent: 0 });
    } finally {
      setTesting(false);
    }
  };

  const valid = name.trim().length > 0 && address.trim().length > 0;

  if (!open) return null;
  return (
    <Modal title={t("executors.addTitle")} onClose={onClose}>
      <div className="space-y-3">
        <Field label={t("executors.nameLabel")} required>
          <TextInput value={name} onChange={setName} autoFocus placeholder={t("executors.namePlaceholder")} />
        </Field>
        <Field label={t("executors.addressLabel")} required hint={t("executors.addressHint")}>
          <TextInput value={address} onChange={setAddress} mono placeholder="192.168.1.50:47777" />
        </Field>
        <Field label={t("executors.tokenLabel")} hint={t("executors.tokenHint")}>
          <TextInput value={token} onChange={setToken} mono placeholder={t("executors.tokenPlaceholder")} />
        </Field>
        <ToggleRowLocal
          label={t("executors.tls")}
          description={t("executors.tlsDescription")}
          on={useTLS}
          onChange={setUseTLS}
        />
        {testResult && (
          <p className={cn("text-[11.5px]", testResult.online ? "text-success-fg" : "text-danger-fg")}>
            {testResult.online
              ? t("executors.testOk", { version: testResult.version ?? "", platform: testResult.platform ?? "" })
              : t("executors.testFail")}
          </p>
        )}
      </div>
      <div className="ml-auto flex items-center gap-2">
        <Button variant="ghost" icon="PlugZap" disabled={!valid || testing} onClick={() => void test()}>
          {testing ? t("common.loading") : t("executors.test")}
        </Button>
        <Button variant="ghost" onClick={onClose}>
          {t("common.cancel")}
        </Button>
        <Button variant="primary" icon="Check" disabled={!valid || saving} onClick={() => void create()}>
          {saving ? t("common.saving") : t("common.save")}
        </Button>
      </div>
    </Modal>
  );
}

function ConfigureExecutorDialog({
  executorId,
  onClose,
  onSaved,
  onError,
}: {
  executorId: string | null;
  onClose: () => void;
  onSaved: () => void;
  onError: () => void;
}) {
  const { t } = useTranslation();
  const [config, setConfig] = useState<RemoteExecutorConfig | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!executorId) {
      setConfig(null);
      return;
    }
    void desktop
      .getRemoteExecutorConfig(executorId)
      .then(setConfig)
      .catch(() => {
        onError();
        onClose();
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [executorId]);

  if (!executorId || !config) return null;

  const patch = (p: Partial<RemoteExecutorConfig>) => setConfig({ ...config, ...p });

  const patchProvider = (id: string, p: Partial<RemoteExecutorProvider>) =>
    patch({ providers: config.providers.map((provider) => (provider.id === id ? { ...provider, ...p } : provider)) });

  const addProvider = () =>
    patch({
      providers: [
        ...config.providers,
        {
          id: `provider-${Date.now().toString(36)}`,
          name: "",
          kind: "openai-compatible" as ProviderKind,
          baseUrl: "",
          model: "",
          enabled: true,
          apiKey: "",
          apiKeySet: false,
        },
      ],
    });

  const save = async () => {
    setSaving(true);
    try {
      await desktop.updateRemoteExecutorConfig(executorId, config);
      onSaved();
      onClose();
    } catch {
      onError();
    } finally {
      setSaving(false);
    }
  };

  const providerOptions = PROVIDER_KINDS.map((kind) => ({ value: kind, label: t(`executors.kind.${kind}`) }));
  const defaultOptions = [
    { value: "", label: t("executors.defaultProviderNone") },
    ...config.providers.filter((p) => p.enabled).map((p) => ({ value: p.id, label: p.name || p.id })),
  ];

  return (
    <Modal title={t("executors.configureTitle")} onClose={onClose}>
      <div className="space-y-4">
        <Field label={t("executors.llmMode")} hint={t(config.llmMode === "local" ? "executors.llmModeLocalHint" : "executors.llmModeProxyHint")}>
          <Dropdown
            value={config.llmMode}
            onChange={(v) => patch({ llmMode: v as ExecutorLLMMode })}
            options={[
              { value: "proxy", label: t("executors.modeProxy") },
              { value: "local", label: t("executors.modeLocal") },
            ]}
          />
        </Field>

        {config.llmMode === "local" && (
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-[11.5px] font-medium text-fg-subtle">{t("executors.providers")}</span>
              <Button variant="ghost" icon="Plus" onClick={addProvider}>
                {t("executors.providerAdd")}
              </Button>
            </div>
            {config.providers.map((provider) => (
              <div key={provider.id} className="space-y-2 rounded-lg border border-seam p-2.5">
                <div className="flex items-center gap-2">
                  <TextInput
                    value={provider.name}
                    onChange={(v) => patchProvider(provider.id, { name: v })}
                    placeholder={t("executors.providerName")}
                  />
                  <Dropdown
                    compact
                    className="w-[150px]"
                    value={provider.kind}
                    onChange={(v) => patchProvider(provider.id, { kind: v })}
                    options={[...providerOptions]}
                  />
                  <Toggle on={provider.enabled} onChange={(v) => patchProvider(provider.id, { enabled: v })} />
                </div>
                <div className="flex items-center gap-2">
                  <TextInput
                    mono
                    value={provider.baseUrl}
                    onChange={(v) => patchProvider(provider.id, { baseUrl: v })}
                    placeholder={t("executors.providerUrl")}
                  />
                  <TextInput
                    value={provider.model}
                    onChange={(v) => patchProvider(provider.id, { model: v })}
                    placeholder={t("executors.providerModel")}
                  />
                </div>
                <div className="flex items-center gap-2">
                  <TextInput
                    mono
                    type="password"
                    value={provider.apiKey ?? ""}
                    onChange={(v) => patchProvider(provider.id, { apiKey: v })}
                    placeholder={
                      provider.apiKeySet ? t("executors.apiKeyReplace") : t("executors.apiKeyPlaceholder")
                    }
                  />
                  <Button
                    variant="ghost"
                    icon="Trash2"
                    onClick={() => patch({ providers: config.providers.filter((p) => p.id !== provider.id) })}
                  >
                    {t("common.delete")}
                  </Button>
                </div>
              </div>
            ))}
            <Field label={t("executors.defaultProvider")}>
              <Dropdown
                value={config.defaultProviderId}
                onChange={(v) => patch({ defaultProviderId: v })}
                options={defaultOptions}
              />
            </Field>
          </div>
        )}

        <Field label={t("executors.maxConcurrent")} hint={t("executors.maxConcurrentHint")}>
          <TextInput
            type="number"
            value={String(config.maxConcurrentRuns)}
            onChange={(v) => patch({ maxConcurrentRuns: Math.max(1, Number(v) || 1) })}
          />
        </Field>
      </div>
      <div className="ml-auto flex items-center gap-2">
        <Button variant="ghost" onClick={onClose}>
          {t("common.cancel")}
        </Button>
        <Button variant="primary" icon="Check" disabled={saving} onClick={() => void save()}>
          {saving ? t("common.saving") : t("common.save")}
        </Button>
      </div>
    </Modal>
  );
}

function TokenOnceModal({
  title,
  description,
  token,
  onClose,
}: {
  title: string;
  description: string;
  token: string | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  if (!token) return null;
  return (
    <Modal title={title} onClose={onClose}>
      <p className="text-[12.5px] leading-relaxed text-fg-subtle">{description}</p>
      <code className="mt-3 block break-all rounded-lg border border-seam bg-ink-900 p-2.5 font-mono text-[12px] text-success-fg">
        {token}
      </code>
      <div className="ml-auto flex items-center gap-2">
        <Button
          variant="ghost"
          icon="Copy"
          onClick={() => void navigator.clipboard.writeText(token).then(onClose)}
        >
          {t("common.copy")}
        </Button>
        <Button variant="primary" onClick={onClose}>
          {t("common.close")}
        </Button>
      </div>
    </Modal>
  );
}

function ToggleRowLocal({
  label,
  description,
  on,
  onChange,
}: {
  label: string;
  description?: string;
  on: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div className="flex items-start justify-between gap-3">
      <span className="min-w-0">
        <span className="block text-[12.5px] text-fg">{label}</span>
        {description && <span className="mt-0.5 block text-[11px] leading-snug text-fg-faint">{description}</span>}
      </span>
      <Toggle on={on} onChange={onChange} />
    </div>
  );
}

