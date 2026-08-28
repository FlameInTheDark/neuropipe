import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Browser } from "@wailsio/runtime";
import i18n from "@/i18n";
import { desktop } from "@/lib/bridge";
import type {
  APIStatus,
  InstallProgress,
  LlamaRuntimeCatalogStatus,
  LlamaRuntimeRelease,
  LlamaRuntimeSettings,
  LlamaRuntimeStatus,
  LocalModel,
  ModelDetail,
  ModelSearchRequest,
  ModelSearchResult,
  PluginStatus,
  ProviderConfig,
  RuntimeMode,
  SecretMetadata,
  Settings,
  TriggerBinding,
  TwitchDeviceAuthorization,
  TwitchEventDescriptor,
  TwitchIdentity,
  TwitchManualIdentityRequest,
  TwitchStatus,
  DiscordEventDescriptor,
  DiscordIdentity,
  DiscordStatus,
  TelegramIdentity,
  TelegramStatus,
} from "@/lib/types";
import { formatBytes, formatCompact, formatDateTime } from "@/lib/format";
import type { Workspace } from "@/features/workspace/useWorkspace";
import { ask } from "@/stores/confirmation";
import { useThemeStore } from "@/stores/theme";
import { Card, ViewShell, EmptyState } from "../components/ViewShell";
import { Button, Toggle } from "../components/ui";
import { Icon } from "../components/icons";
import { Dropdown } from "../components/Dropdown";
import { Modal, ModalActions } from "../components/primitives/Modal";
import { Field, TextInput, TextArea } from "../components/primitives/Field";
import { RemoteExecutorsPanel } from "./RemoteExecutorsPanel";
import { cn } from "../utils/cn";

const SECTIONS = [
  { id: "general", labelKey: "settings.general", icon: "Settings2" },
  { id: "provider", labelKey: "settings.provider", icon: "Cable" },
  { id: "models", labelKey: "settings.models", icon: "HardDrive" },
  { id: "runtime", labelKey: "settings.runtime", icon: "Activity" },
  { id: "executors", labelKey: "executors.title", icon: "Server" },
  { id: "api", labelKey: "settings.api", icon: "Radio" },
  { id: "twitch", labelKey: "twitch.title", icon: "Radio" },
  { id: "discord", labelKey: "discord.title", icon: "Hash" },
  { id: "telegram", labelKey: "telegram.title", icon: "Send" },
  { id: "execution", labelKey: "settings.execution", icon: "Play" },
  { id: "metrics", labelKey: "settings.metrics", icon: "Activity" },
  { id: "extensions", labelKey: "settings.extensions", icon: "Sparkles" },
  { id: "secrets", labelKey: "settings.secrets", icon: "KeyRound" },
] as const;

type SectionId = (typeof SECTIONS)[number]["id"];

/** How often integration panels re-read live service state while open. The
 *  gateway/polling loops connect asynchronously after trust/enable/add-bot
 *  actions, so a one-shot fetch would show stale state seconds later. */
const PANEL_POLL_MS = 5000;

interface IdentitySlice<I> {
  identities: I[];
  defaultBotIdentityId?: string;
}

/** Adopts backend-managed identity state into a dirty settings draft without
 *  touching the user's unsaved edits: the identity list always follows the
 *  backend (the UI only mutates it through backend calls), while the default
 *  selection keeps the user's choice unless it is unset or no longer points
 *  at an existing identity (removed bot), in which case the backend's
 *  rotation wins. */
function mergeIdentitySlice<S extends IdentitySlice<{ id: string }>>(draft: S, backend: S): S {
  const draftDefault = draft.defaultBotIdentityId ?? "";
  const backendDefault = backend.defaultBotIdentityId ?? "";
  const draftDefaultValid =
    draftDefault !== "" && backend.identities.some((identity) => identity.id === draftDefault);
  const nextDefault = draftDefaultValid ? draftDefault : backendDefault;
  return {
    ...draft,
    identities: backend.identities,
    defaultBotIdentityId: nextDefault || undefined,
  };
}

/** Normalises a loaded Settings object before it enters the editor draft. */
function normalizeSettings(input: Settings): Settings {
  const providers = input.providers.length > 0 ? input.providers : [defaultProvider("openai-compatible")];
  return {
    ...input,
    language: ["en", "de", "fr", "ru"].includes(input.language) ? input.language : "en",
    api: { ...input.api, port: input.api.port || input.webhookPort || 7878 },
    llamaRuntime: {
      ...input.llamaRuntime,
      binaryPath: input.llamaRuntime.binaryPath ?? "",
      modelPath: input.llamaRuntime.modelPath ?? "",
      mode: input.llamaRuntime.mode ?? ("auto" as const),
      contextSize: input.llamaRuntime.contextSize ?? 8192,
      autoStart: input.llamaRuntime.autoStart ?? false,
    },
    metrics: {
      ...input.metrics,
      detailRetentionDays: input.metrics.detailRetentionDays ?? 30,
      rollupRetentionDays: input.metrics.rollupRetentionDays ?? 365,
      sampleIntervalSeconds: input.metrics.sampleIntervalSeconds ?? 30,
      priceRates: input.metrics.priceRates ?? [],
    },
    twitch: { ...input.twitch, identities: input.twitch?.identities ?? [], clientId: input.twitch?.clientId ?? "" },
    discord: { ...input.discord, identities: input.discord?.identities ?? [] },
    telegram: { ...input.telegram, identities: input.telegram?.identities ?? [] },
    providers,
  };
}

function defaultProvider(kind: ProviderConfig["kind"]): ProviderConfig {
  switch (kind) {
    case "ollama":
      return { id: "ollama-local", name: "Ollama (local)", kind, baseUrl: "http://127.0.0.1:11434", model: "", enabled: true };
    case "llamacpp":
      return { id: "llama-managed", name: "Managed llama.cpp", kind, baseUrl: "", model: "", enabled: true };
    default:
      // No example URL here on purpose: an empty field plus its placeholder
      // must never look like configuration that was loaded from settings.
      return { id: "openai-compatible", name: "OpenAI-compatible", kind, baseUrl: "", model: "", enabled: true };
  }
}

export function SettingsView({ workspace }: { workspace: Workspace }) {
  const { t } = useTranslation();
  const [section, setSection] = useState<SectionId>("general");
  const [draft, setDraft] = useState<Settings | null>(
    workspace.settings ? normalizeSettings(workspace.settings) : null,
  );
  const [saving, setSaving] = useState(false);
  /* local edits must survive background workspace refreshes; the ref clears
     after a successful save so external changes flow in again. */
  const dirtyRef = useRef(false);

  /* re-sync when the workspace loads/changes settings externally */
  useEffect(() => {
    if (!workspace.settings || saving) return;
    const next = normalizeSettings(workspace.settings);
    setDraft((d) => {
      if (!d || !dirtyRef.current) return next; // clean draft: adopt wholesale
      // Dirty draft: unsaved edits survive, but backend-managed identity
      // state (added/removed bots, default rotation) always flows in so the
      // integration panels never show stale identities.
      return {
        ...d,
        twitch: mergeIdentitySlice(d.twitch, next.twitch),
        discord: mergeIdentitySlice(d.discord, next.discord),
        telegram: mergeIdentitySlice(d.telegram, next.telegram),
      };
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace.settings, saving]);

  if (!draft) {
    return (
      <ViewShell title={t("nav.settings")} subtitle={t("settings.description")}>
        <EmptyState icon="AlertTriangle" title={t("common.unavailable")} />
      </ViewShell>
    );
  }

  const patch = (p: Partial<Settings>) => {
    dirtyRef.current = true;
    setDraft((d) => (d ? { ...d, ...p } : d));
  };

  const save = async () => {
    if (!draft || saving) return;
    // exposure handshake: non-loopback or unauthenticated API listeners need
    // explicit consent before they are persisted.
    const api = draft.api;
    const isLoopback = api.bindAddress === "127.0.0.1" || api.bindAddress === "::1";
    const risky = api.enabled && (!isLoopback || api.authMode === "none");
    let next = draft;
    if (risky && !api.exposureAcknowledged) {
      const ok = await ask({
        title: t("api.exposureTitle"),
        description: t("api.exposureDescription"),
        confirmLabel: t("api.exposureConfirm"),
        danger: true,
      });
      if (!ok) return;
      next = { ...draft, api: { ...draft.api, exposureAcknowledged: true } };
      setDraft(next);
    }
    setSaving(true);
    try {
      await workspace.saveSettings(next);
      dirtyRef.current = false; // draft now matches persisted state
      workspace.notify(t("settings.saved"), "Check");
    } catch {
      workspace.notify(t("settings.saveFailed"), "AlertTriangle");
    } finally {
      setSaving(false);
    }
  };

  return (
    <ViewShell
      title={t("nav.settings")}
      subtitle={t("settings.description")}
      padded={false}
      actions={
        <Button icon="Save" variant="primary" onClick={() => void save()} disabled={saving}>
          {saving ? t("common.saving") : t("settings.save")}
        </Button>
      }
    >
      <div className="flex h-full min-h-0">
        <aside className="w-[210px] shrink-0 overflow-y-auto border-r border-seam p-2">
          {SECTIONS.map((s) => (
            <button
              key={s.id}
              onClick={() => setSection(s.id)}
              className={cn(
                "mb-0.5 flex w-full items-center gap-2.5 rounded-lg px-2.5 py-[7px] text-left text-[12.5px] transition",
                section === s.id ? "bg-ink-750 text-fg" : "text-fg-subtle hover:bg-ink-850 hover:text-fg",
              )}
            >
              <Icon name={s.icon} className="h-[15px] w-[15px] shrink-0" />
              <span className="min-w-0 flex-1 truncate">{t(s.labelKey)}</span>
              {section === s.id && <Icon name="ChevronRight" className="h-3.5 w-3.5 text-fg-faint" />}
            </button>
          ))}
        </aside>

        <div className="fade-in min-w-0 flex-1 overflow-y-auto p-5">
          {section === "general" && <GeneralPanel draft={draft} patch={patch} />}
          {section === "provider" && <ProviderPanel draft={draft} patch={patch} notify={workspace.notify} />}
          {section === "models" && <ModelsPanel draft={draft} patch={patch} notify={workspace.notify} />}
          {section === "runtime" && (
            <RuntimePanel
              draft={draft}
              patch={patch}
              notify={workspace.notify}
              onSaveDraft={() => workspace.saveSettings(draft)}
            />
          )}
          {section === "api" && <ApiPanel draft={draft} patch={patch} />}
          {section === "executors" && <RemoteExecutorsPanel workspace={workspace} />}
          {section === "twitch" && (
            <TwitchPanel
              draft={draft}
              patch={patch}
              triggers={workspace.triggers}
              refreshTriggers={workspace.refreshTriggers}
              refreshSettings={workspace.refreshSettings}
            />
          )}
          {section === "discord" && (
            <DiscordPanel
              draft={draft}
              patch={patch}
              triggers={workspace.triggers}
              refreshTriggers={workspace.refreshTriggers}
              refreshSettings={workspace.refreshSettings}
            />
          )}
          {section === "telegram" && (
            <TelegramPanel
              draft={draft}
              patch={patch}
              triggers={workspace.triggers}
              refreshTriggers={workspace.refreshTriggers}
              refreshSettings={workspace.refreshSettings}
            />
          )}
          {section === "execution" && <ExecutionPanel draft={draft} patch={patch} />}
          {section === "metrics" && <MetricsPanel draft={draft} patch={patch} />}
          {section === "extensions" && <ExtensionsPanel draft={draft} patch={patch} />}
          {section === "secrets" && <SecretsPanel />}
        </div>
      </div>
    </ViewShell>
  );
}

/* ------------------------------------------------------------------ */
/* shared bits                                                         */
/* ------------------------------------------------------------------ */

export function SectionCard({
  title,
  children,
  action,
}: {
  title: string;
  children: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <Card className="p-4">
      <div className="mb-3 flex items-center gap-2">
        <h3 className="text-[12.5px] font-semibold tracking-wide text-fg uppercase">{title}</h3>
        {action && <div className="ml-auto">{action}</div>}
      </div>
      {children}
    </Card>
  );
}

function ToggleRow({
  title,
  description,
  on,
  onChange,
}: {
  title: string;
  description: string;
  on: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <Card className="flex items-center gap-3 p-3.5">
      <span className="min-w-0 flex-1">
        <span className="block text-[12.5px] font-medium text-fg">{title}</span>
        <span className="mt-0.5 block text-[11.5px] text-fg-faint">{description}</span>
      </span>
      <Toggle on={on} onChange={onChange} />
    </Card>
  );
}

function NumberInput({
  value,
  onChange,
  min,
  max,
}: {
  value: number;
  onChange: (v: number) => void;
  min?: number;
  max?: number;
}) {
  return (
    <input
      type="number"
      value={Number.isFinite(value) ? value : ""}
      min={min}
      max={max}
      onChange={(e) => {
        const n = Number(e.target.value);
        if (!Number.isNaN(n)) onChange(Math.min(max ?? Number.MAX_SAFE_INTEGER, Math.max(min ?? 0, n)));
      }}
      className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
    />
  );
}

/* ------------------------------------------------------------------ */
/* general                                                             */
/* ------------------------------------------------------------------ */

function GeneralPanel({
  draft,
  patch,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
}) {
  const { t } = useTranslation();
  const theme = useThemeStore((s) => s.theme);
  const setTheme = useThemeStore((s) => s.setTheme);

  const changeLanguage = async (language: string) => {
    await i18n.changeLanguage(language);
    patch({ language: language as Settings["language"] });
  };

  const browse = async () => {
    try {
      const dir = await desktop.chooseContentDirectory();
      if (dir) patch({ contentDirectory: dir });
    } catch {
      /* picker canceled */
    }
  };

  return (
    <div className="mx-auto max-w-[640px] space-y-3">
      <SectionCard title={t("settings.languageTitle")}>
        <Field label={t("common.language")} hint={t("settings.languageDescription")}>
          <Dropdown
            value={draft.language}
            onChange={(v) => void changeLanguage(v)}
            options={[
              { value: "en", label: t("common.english") },
              { value: "de", label: t("common.german") },
              { value: "fr", label: t("common.french") },
              { value: "ru", label: t("common.russian") },
            ]}
          />
        </Field>
      </SectionCard>

      <SectionCard title={t("settings.appearanceTitle")}>
        <Field label={t("common.theme")} hint={t("settings.themeDescription")}>
          <Dropdown
            value={theme}
            onChange={(v) => setTheme(v as "dark" | "light")}
            options={[
              { value: "dark", label: t("common.themeDark") },
              { value: "light", label: t("common.themeLight") },
            ]}
          />
        </Field>
      </SectionCard>

      <ToggleRow
        title={t("settings.hideToTrayOnCloseTitle")}
        description={t("settings.hideToTrayOnCloseDescription")}
        on={draft.hideToTrayOnClose}
        onChange={(v) => patch({ hideToTrayOnClose: v })}
      />

      <SectionCard title={t("runtime.contentDirectory")}>
        <Field label={t("datastores.workingDirHint")}>
          <div className="flex gap-2">
            <TextInput mono value={draft.contentDirectory} onChange={(v) => patch({ contentDirectory: v })} />
            <Button icon="HardDrive" variant="solid" onClick={() => void browse()}>
              {t("databases.chooseFile")}
            </Button>
          </div>
        </Field>
      </SectionCard>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* provider                                                            */
/* ------------------------------------------------------------------ */

function ProviderPanel({
  draft,
  patch,
  notify,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
  notify: (text: string, icon?: string) => void;
}) {
  const { t } = useTranslation();
  const [secrets, setSecrets] = useState<string[]>([]);

  useEffect(() => {
    desktop
      .listSecrets()
      .then((list) => setSecrets(list.map((s) => s.name)))
      .catch(() => notify(t("settings.secretsLoadFailed"), "AlertTriangle"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const provider = draft.providers[0];

  const selectProvider = (kind: ProviderConfig["kind"]) => {
    const next = defaultProvider(kind);
    patch({ providers: [next], defaultProviderId: next.id });
  };

  const updateProvider = (key: "baseUrl" | "model" | "apiKeyRef", value: string) => {
    patch({
      providers: draft.providers.map((p) =>
        p.id === provider.id ? { ...p, [key]: value || undefined } : p,
      ),
    });
  };

  if (!provider) return null;

  return (
    <div className="mx-auto max-w-[720px]">
      <SectionCard title={t("settings.providerHelp")}>
        <div className="grid grid-cols-2 gap-3">
          <Field label={t("provider.kind")}>
            <Dropdown
              value={provider.kind}
              onChange={(v) => selectProvider(v as ProviderConfig["kind"])}
              options={[
                { value: "openai-compatible", label: t("provider.openaiCompatible") },
                { value: "ollama", label: t("provider.ollama") },
                { value: "llamacpp", label: t("provider.managedLlamaCpp") },
              ]}
            />
          </Field>
          {provider.kind !== "llamacpp" ? (
            <>
              <Field label={t("provider.baseUrl")}>
                <TextInput
                  value={provider.baseUrl}
                  onChange={(v) => updateProvider("baseUrl", v)}
                  mono
                  placeholder={provider.kind === "ollama" ? "http://127.0.0.1:11434" : "https://api.example.com/v1"}
                />
              </Field>
              <Field label={t("provider.model")}>
                <TextInput value={provider.model} onChange={(v) => updateProvider("model", v)} />
              </Field>
              {provider.kind === "openai-compatible" && (
                <Field label={t("provider.apiKeyRef")} hint={t("settings.apiKeyHelp")}>
                  <Dropdown
                    value={provider.apiKeyRef ?? ""}
                    onChange={(v) => updateProvider("apiKeyRef", v)}
                    placeholder={t("settings.noApiKey")}
                    options={[
                      { value: "", label: t("settings.noApiKey") },
                      ...secrets.map((name) => ({ value: name, label: name, icon: "KeyRound" })),
                    ]}
                  />
                </Field>
              )}
            </>
          ) : (
            <p className="col-span-2 self-end rounded-md border border-ink-700 bg-ink-900/60 px-3 py-2 text-[11.5px] leading-relaxed text-fg-subtle">
              {t("provider.llamacppNote")}
            </p>
          )}
        </div>
      </SectionCard>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* models                                                              */
/* ------------------------------------------------------------------ */

function ModelsPanel({
  draft,
  patch,
  notify,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
  notify: (text: string, icon?: string) => void;
}) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<"catalog" | "installed">("catalog");
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<ModelSearchRequest["sort"]>("recommended");
  const [results, setResults] = useState<ModelSearchResult[]>([]);
  const [detail, setDetail] = useState<ModelDetail | null>(null);
  const [selectedFile, setSelectedFile] = useState<string>("");
  const [installed, setInstalled] = useState<LocalModel[]>([]);
  const [busy, setBusy] = useState<string | null>(null);
  const [progress, setProgress] = useState<InstallProgress | null>(null);
  /** Installed model opened in the details pane. */
  const [infoModel, setInfoModel] = useState<LocalModel | null>(null);

  const search = useCallback(
    async (q: string) => {
      setBusy("search");
      try {
        const list = await desktop.searchLlamaModels({ query: q.trim(), sort });
        setResults(list);
        if (list[0]) await loadDetail(list[0].id);
      } catch {
        notify(t("models.searchFailed"), "AlertTriangle");
      } finally {
        setBusy(null);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [sort],
  );

  const loadDetail = async (repository: string) => {
    setBusy(`detail-${repository}`);
    try {
      const d = await desktop.getLlamaModelDetail(repository);
      setDetail(d);
      setSelectedFile(d.files.find((f) => f.recommended)?.name ?? d.files[0]?.name ?? "");
    } catch {
      /* keep previous detail */
    } finally {
      setBusy(null);
    }
  };

  const refreshInstalled = useCallback(async () => {
    const list = await desktop.listInstalledLlamaModels().catch(() => [] as LocalModel[]);
    setInstalled(list);
  }, []);

  useEffect(() => {
    void refreshInstalled();
  }, [refreshInstalled]);

  /* install progress arrives via push events and is polled while busy */
  useEffect(() => {
    const off = (window as unknown as { __wailsEvents?: unknown }).__wailsEvents; // no-op guard for plain-browser runs
    void off;
    const timer = window.setInterval(async () => {
      if (busy !== "model-install") return;
      try {
        setProgress(await desktop.getInstallProgress("model"));
      } catch {
        /* ignore */
      }
    }, 400);
    return () => window.clearInterval(timer);
  }, [busy]);

  const installModel = async () => {
    if (!detail || !selectedFile) return;
    setBusy("model-install");
    setProgress({ kind: "model", stage: "preparing", label: t("models.preparing"), downloadedBytes: 0, totalBytes: 0, bytesPerSecond: 0, percentage: 0 });
    try {
      await desktop.installLlamaModel({ repository: detail.id, file: selectedFile });
      await refreshInstalled();
      notify(t("models.installed"), "Check");
    } catch {
      notify(t("models.installFailed"), "AlertTriangle");
    } finally {
      setBusy(null);
      setProgress(null);
    }
  };

  const selectInstalled = async (path: string) => {
    await desktop.selectInstalledLlamaModel(path).catch(() => undefined);
    patch({ llamaRuntime: { ...draft.llamaRuntime, modelPath: path } });
  };

  const deleteInstalled = async (model: LocalModel) => {
    const ok = await ask({
      title: t("models.deleteTitle"),
      description: t("models.deleteDescription", { name: model.name }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    await desktop.deleteInstalledLlamaModel(model.path).catch(() => undefined);
    setInfoModel((cur) => (cur?.path === model.path ? null : cur));
    await refreshInstalled();
  };

  const openOnHf = async (repo: string) => {
    try {
      await Browser.OpenURL(`https://huggingface.co/${repo}`);
    } catch {
      /* outside Wails */
    }
  };

  return (
    <div className="grid h-full min-h-[520px] grid-cols-[minmax(300px,0.85fr)_minmax(420px,1.15fr)] gap-3">
      {/* catalog browser */}
      <div className="flex min-h-0 flex-col overflow-hidden rounded-xl border border-ink-700/80 bg-ink-900/70">
        <div className="border-b border-seam p-3">
          <div className="mb-3 grid grid-cols-2 gap-1 rounded-lg border border-ink-700 bg-ink-950 p-1">
            {(["catalog", "installed"] as const).map((tabValue) => (
              <button
                key={tabValue}
                onClick={() => {
                  setMode(tabValue);
                  if (tabValue === "catalog") setInfoModel(null);
                }}
                className={cn(
                  "rounded-md px-2 py-1.5 text-[12px] font-medium transition",
                  mode === tabValue ? "bg-ink-700 text-fg" : "text-fg-subtle hover:text-fg",
                )}
              >
                {tabValue === "installed" ? `${t("models.installedTab")} (${installed.length})` : t("models.catalogTab")}
              </button>
            ))}
          </div>

          {mode === "catalog" && (
            <>
              <div className="mt-2 grid grid-cols-[minmax(0,1fr)_140px] gap-2">
                <input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && void search(query)}
                  placeholder={t("models.searchPlaceholder")}
                  aria-label={t("models.searchPlaceholder")}
                  className="flex h-8 w-full items-center gap-2 rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-fg placeholder:text-fg-faint focus:border-ink-500"
                />
                <Dropdown
                  value={sort}
                  onChange={(v) => setSort(v as ModelSearchRequest["sort"])}
                  options={[
                    { value: "recommended", label: t("models.sortRecommended"), icon: "Sparkles" },
                    { value: "downloads", label: t("models.sortDownloads"), icon: "TrendingUp" },
                    { value: "recent", label: t("models.sortRecent"), icon: "History" },
                  ]}
                />
              </div>
              <Button icon="Search" variant="solid" className="mt-2 w-full justify-center" onClick={() => void search(query)} disabled={busy === "search"}>
                {busy === "search" ? t("common.loading") : t("common.search")}
              </Button>
            </>
          )}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto p-2">
          {mode === "catalog"
            ? results.map((m) => (
                <button
                  key={m.id}
                  onClick={() => void loadDetail(m.id)}
                  className={cn(
                    "group mb-1.5 flex w-full items-start gap-2.5 rounded-xl border px-2.5 py-2.5 text-left transition",
                    detail?.id === m.id
                      ? "border-ink-400 bg-ink-800 shadow-[0_0_0_1px_rgba(236,237,241,0.08)_inset]"
                      : "border-transparent hover:border-ink-700 hover:bg-ink-850/70",
                  )}
                >
                  <ModelAvatar id={m.id} author={m.author} avatarUrl={m.avatarUrl} className="mt-[1px] h-9 w-9 text-[13px]" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[12.5px] font-semibold text-fg">{m.id}</span>
                    <span className="mt-0.5 block truncate text-[11px] text-fg-faint">
                      {m.author} · ↓{formatCompact(m.downloads)} · ♥{formatCompact(m.likes)}
                    </span>
                  </span>
                  <Icon name="ChevronRight" className="mt-1 h-4 w-4 shrink-0 text-fg-faint group-hover:text-fg-subtle" />
                </button>
              ))
            : installed.map((model) => (
                <div
                  key={model.path}
                  role="button"
                  tabIndex={0}
                  aria-label={`${t("models.infoAction")}: ${model.name}`}
                  onClick={() => setInfoModel(model)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      setInfoModel(model);
                    }
                  }}
                  className={cn(
                    "group mb-1.5 flex w-full cursor-pointer items-start gap-2.5 rounded-xl border px-2.5 py-2.5 text-left transition focus-visible:border-ink-400 focus-visible:outline-none",
                    draft.llamaRuntime.modelPath === model.path || infoModel?.path === model.path
                      ? "border-ink-400 bg-ink-800"
                      : "border-transparent hover:border-ink-700 hover:bg-ink-850/70",
                  )}
                >
                  <ModelAvatar
                    id={model.repository || model.name}
                    author={model.author ?? model.repository?.split("/")[0]}
                    avatarUrl={model.avatarUrl}
                    className="mt-[1px] h-9 w-9 text-[13px]"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[12.5px] font-semibold text-fg">{model.name}</span>
                    <span className="mt-0.5 block truncate font-mono text-[10.5px] text-fg-faint">{model.path}</span>
                    <span className="mt-0.5 block text-[10.5px] text-fg-faint">
                      {formatBytes(model.size)} · {formatDateTime(model.installedAt)}
                    </span>
                  </span>
                  {draft.llamaRuntime.modelPath === model.path ? (
                    <Icon name="Check" className="mt-1 h-4 w-4 shrink-0 text-success-fg" />
                  ) : (
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        void deleteInstalled(model);
                      }}
                      aria-label={t("common.delete")}
                      className="grid h-6 w-6 shrink-0 place-items-center rounded text-fg-faint opacity-0 transition hover:bg-danger/15 hover:text-danger-fg focus-visible:opacity-100 group-hover:opacity-100"
                    >
                      <Icon name="Trash2" className="h-3.5 w-3.5" />
                    </button>
                  )}
                </div>
              ))}
          {((mode === "catalog" && results.length === 0) || (mode === "installed" && installed.length === 0)) && (
            <div className="flex flex-col items-center justify-center gap-2 px-4 py-10 text-center">
              <Icon name="Search" className="h-5 w-5 text-fg-faint" />
              <p className="text-[12px] text-fg-faint">
                {mode === "catalog" ? t("models.searchEmpty") : t("models.noInstalled")}
              </p>
            </div>
          )}
        </div>
      </div>

      {/* selected model details */}
      <div className="flex min-h-0 flex-col overflow-hidden rounded-xl border border-ink-700/80 bg-ink-900/70">
        {mode === "installed" ? (
          infoModel ? (
            <InstalledModelInfo
              model={infoModel}
              active={draft.llamaRuntime.modelPath === infoModel.path}
              onSelect={() => void selectInstalled(infoModel.path)}
              onDelete={() => void deleteInstalled(infoModel)}
              onOpenHf={infoModel.repository ? () => void openOnHf(infoModel.repository!) : undefined}
            />
          ) : (
            <div className="flex flex-1 items-center justify-center">
              <EmptyState icon="HardDrive" title={t("models.pickModel")} hint={t("models.pickInstalledHint")} />
            </div>
          )
        ) : !detail ? (
          <div className="flex flex-1 items-center justify-center">
            <EmptyState icon="HardDrive" title={t("models.pickModel")} hint={t("models.pickModelHint")} />
          </div>
        ) : (
          <>
            <div className="border-b border-seam p-4">
              <div className="flex items-start gap-3">
                <ModelAvatar id={detail.id} author={detail.author} avatarUrl={detail.avatarUrl} className="h-12 w-12 rounded-2xl text-[18px]" />
                <div className="min-w-0 flex-1">
                  <h2 className="truncate text-[16px] font-semibold tracking-tight text-fg">{detail.id}</h2>
                  <p className="mt-1 truncate font-mono text-[11px] text-fg-faint">
                    {detail.author}/{detail.id.split("/").pop()}
                  </p>
                  <div className="mt-2 flex flex-wrap gap-1.5 text-[10.5px] text-fg-subtle">
                    <span className="rounded-md border border-ink-700 bg-ink-850 px-2 py-1">↓ {formatCompact(detail.downloads)}</span>
                    <span className="rounded-md border border-ink-700 bg-ink-850 px-2 py-1">♥ {formatCompact(detail.likes)}</span>
                    {detail.lastModified && (
                      <span className="rounded-md border border-ink-700 bg-ink-850 px-2 py-1">{formatDateTime(detail.lastModified)}</span>
                    )}
                  </div>
                </div>
                <Button icon="ExternalLink" variant="solid" onClick={() => void openOnHf(detail.id)}>
                  {t("models.openHf")}
                </Button>
              </div>
            </div>

            <div className="min-h-0 flex-1 overflow-y-auto p-4">
              <div className="rounded-xl border border-ink-700/80 bg-ink-850/40 p-3.5">
                <h3 className="mb-2 text-[11px] font-medium tracking-[0.08em] text-fg-subtle uppercase">{t("models.installPackage")}</h3>
                <div className="grid grid-cols-[1fr_130px] gap-2">
                  <Dropdown
                    value={selectedFile}
                    onChange={setSelectedFile}
                    options={detail.files.slice(0, 40).map((f) => ({
                      value: f.name,
                      label: `${f.name}${f.size ? ` · ${formatBytes(f.size)}` : ""}`,
                      icon: "HardDrive",
                    }))}
                  />
                  <Button
                    icon={busy === "model-install" ? "Loader2" : "Download"}
                    spin={busy === "model-install"}
                    variant="primary"
                    onClick={() => void installModel()}
                    disabled={!selectedFile || busy === "model-install"}
                  >
                    {t("models.install")}
                  </Button>
                </div>
                {progress && busy === "model-install" && (
                  <div className="mt-2">
                    <ProgressBar progress={progress} />
                  </div>
                )}
              </div>

              {detail.readme && (
                <div className="mt-4 rounded-xl border border-ink-700/80 bg-ink-850/40 p-3.5">
                  <h3 className="mb-2 text-[11px] font-medium tracking-[0.08em] text-fg-subtle uppercase">README</h3>
                  <pre className="max-h-[240px] overflow-auto rounded-lg border border-ink-700 bg-ink-950/50 p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-fg-subtle">
                    {detail.readme}
                  </pre>
                </div>
              )}
            </div>

            <div className="flex h-9 shrink-0 items-center border-t border-seam px-4 text-[11px] text-fg-faint">
              <span className="ml-auto truncate">{draft.contentDirectory}</span>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

/** Hugging Face account avatar with a letter fallback. Broken or missing
 *  images degrade back to the initial so rows never show empty frames. */
function ModelAvatar({
  id,
  author,
  avatarUrl,
  className,
}: {
  id: string;
  author?: string;
  avatarUrl?: string;
  className?: string;
}) {
  const [broken, setBroken] = useState(false);
  return (
    <span
      className={cn(
        "relative grid shrink-0 place-items-center overflow-hidden rounded-lg border border-success/20 bg-success/10 font-semibold text-success-fg",
        className,
      )}
    >
      {(author ?? id).slice(0, 1).toUpperCase()}
      {avatarUrl && !broken && (
        <img
          src={avatarUrl}
          alt=""
          referrerPolicy="no-referrer"
          loading="lazy"
          onError={() => setBroken(true)}
          className="absolute inset-0 h-full w-full object-cover"
        />
      )}
    </span>
  );
}

/** Details pane for one installed local model: identity, file facts, and
 *  the use / open / delete actions. */
function InstalledModelInfo({
  model,
  active,
  onSelect,
  onDelete,
  onOpenHf,
}: {
  model: LocalModel;
  active: boolean;
  onSelect: () => void;
  onDelete: () => void;
  onOpenHf?: () => void;
}) {
  const { t } = useTranslation();
  const tags = (model.tags ?? []).filter(Boolean).slice(0, 8);
  const rows: Array<{ label: string; value: string; mono?: boolean }> = [
    { label: t("models.detailsPath"), value: model.path, mono: true },
    ...(model.quantization ? [{ label: t("models.detailsQuantization"), value: model.quantization }] : []),
    ...(model.sha256 ? [{ label: t("models.detailsSha"), value: model.sha256, mono: true }] : []),
    ...(model.installedAt ? [{ label: t("models.detailsInstalled"), value: formatDateTime(model.installedAt) }] : []),
    ...(model.lastModified ? [{ label: t("models.detailsModified"), value: formatDateTime(model.lastModified) }] : []),
  ];
  return (
    <>
      <div className="border-b border-seam p-4">
        <div className="flex items-start gap-3">
          <ModelAvatar
            id={model.repository || model.name}
            author={model.author ?? model.repository?.split("/")[0]}
            avatarUrl={model.avatarUrl}
            className="h-12 w-12 rounded-2xl text-[18px]"
          />
          <div className="min-w-0 flex-1">
            <h2 className="truncate text-[16px] font-semibold tracking-tight text-fg">{model.name}</h2>
            {model.repository && (
              <p className="mt-1 truncate font-mono text-[11px] text-fg-faint">{model.repository}</p>
            )}
            <div className="mt-2 flex flex-wrap gap-1.5 text-[10.5px] text-fg-subtle">
              <span className="rounded-md border border-ink-700 bg-ink-850 px-2 py-1">{formatBytes(model.size)}</span>
              {model.quantization && (
                <span className="rounded-md border border-ink-700 bg-ink-850 px-2 py-1">{model.quantization}</span>
              )}
            </div>
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-4">
        <div className="rounded-xl border border-ink-700/80 bg-ink-850/40 p-3.5">
          <dl className="space-y-2">
            {rows.map((row) => (
              <div key={row.label} className="grid grid-cols-[110px_minmax(0,1fr)] items-baseline gap-2">
                <dt className="text-[11px] text-fg-faint">{row.label}</dt>
                <dd
                  title={row.value}
                  className={cn(
                    "min-w-0 truncate text-[11.5px] text-fg-muted",
                    row.mono && "font-mono text-[11px]",
                  )}
                >
                  {row.value}
                </dd>
              </div>
            ))}
          </dl>
          {tags.length > 0 && (
            <div className="mt-3 border-t border-seam/70 pt-3">
              <p className="mb-1.5 text-[11px] text-fg-faint">{t("models.detailsTags")}</p>
              <div className="flex flex-wrap gap-1.5">
                {tags.map((tag) => (
                  <span key={tag} className="rounded-md border border-ink-700 bg-ink-850 px-2 py-0.5 text-[10.5px] text-fg-subtle">
                    {tag}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="flex flex-wrap gap-2">
          {active ? (
            <Button variant="primary" icon="Check" disabled>
              {t("models.inUse")}
            </Button>
          ) : (
            <Button variant="primary" icon="Check" onClick={onSelect}>
              {t("models.useModel")}
            </Button>
          )}
          {onOpenHf && (
            <Button variant="solid" icon="ExternalLink" onClick={onOpenHf}>
              {t("models.openHf")}
            </Button>
          )}
          <Button
            variant="ghost"
            icon="Trash2"
            className="text-danger-fg hover:bg-danger/15 hover:text-danger-fg"
            onClick={onDelete}
          >
            {t("common.delete")}
          </Button>
        </div>
      </div>

      <div className="flex h-9 shrink-0 items-center gap-2 border-t border-seam px-4 text-[11px] text-fg-faint">
        {active && (
          <span className="flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-success" />
            {t("models.inUse")}
          </span>
        )}
        <span className="ml-auto truncate">{model.size > 0 ? formatBytes(model.size) : ""}</span>
      </div>
    </>
  );
}

function ProgressBar({ progress }: { progress: InstallProgress }) {
  const pct = Math.max(0, Math.min(100, progress.percentage));
  return (
    <div>
      <div className="mb-1 flex items-center justify-between text-[10.5px] text-fg-subtle">
        <span>{progress.label}</span>
        <span>{Math.round(pct)}%</span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-ink-800">
        <div className="h-full rounded-full bg-ink-100 transition-[width]" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* runtime                                                             */
/* ------------------------------------------------------------------ */

function RuntimePanel({
  draft,
  patch,
  notify,
  onSaveDraft,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
  notify: (text: string, icon?: string) => void;
  onSaveDraft: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<LlamaRuntimeStatus | null>(null);
  const [catalog, setCatalog] = useState<LlamaRuntimeCatalogStatus | null>(null);
  const [releases, setReleases] = useState<LlamaRuntimeRelease[]>([]);
  const [releaseVersion, setReleaseVersion] = useState("");
  const [models, setModels] = useState<LocalModel[]>([]);
  const [busy, setBusy] = useState<string | null>(null);
  const [progress, setProgress] = useState<InstallProgress | null>(null);

  const refresh = useCallback(async () => {
    const [st, cat, rels, mdl] = await Promise.all([
      desktop.getLlamaRuntimeStatus().catch(() => null),
      desktop.getLlamaRuntimeCatalogStatus().catch(() => null),
      desktop.listLlamaRuntimeReleases().catch(() => []),
      desktop.listInstalledLlamaModels().catch(() => []),
    ]);
    setStatus(st);
    setCatalog(cat);
    setReleases(rels);
    setModels(mdl);
    setReleaseVersion((v) => v || rels[0]?.version || "");
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    const timer = window.setInterval(async () => {
      const kind = busy?.startsWith("runtime-") ? "runtime" : null;
      if (!kind) return;
      try {
        setProgress(await desktop.getInstallProgress(kind));
      } catch {
        /* ignore */
      }
    }, 400);
    return () => window.clearInterval(timer);
  }, [busy]);

  const startStop = async () => {
    setBusy("start");
    try {
      await onSaveDraft();
      setStatus(status?.running ? await desktop.stopLlamaRuntime() : await desktop.startLlamaRuntime());
    } catch {
      notify(t("runtime.startStopFailed"), "AlertTriangle");
    } finally {
      setBusy(null);
    }
  };

  const installRuntime = async (mode: Exclude<RuntimeMode, "auto">) => {
    if (!releaseVersion) return;
    setBusy(`runtime-${mode}`);
    try {
      setProgress({ kind: "runtime", stage: "preparing", label: t("runtime.preparing"), downloadedBytes: 0, totalBytes: 0, bytesPerSecond: 0, percentage: 0 });
      const next = await desktop.installLlamaRuntime({ version: releaseVersion, mode });
      setCatalog(next);
      await refresh();
      notify(t("runtime.installed"), "Check");
    } catch {
      notify(t("runtime.installFailed"), "AlertTriangle");
    } finally {
      setBusy(null);
      setProgress(null);
    }
  };

  const rt: LlamaRuntimeSettings = draft.llamaRuntime;

  const versionOptions = useMemo(() => {
    const installedMap = new Map((catalog?.installed ?? []).map((i) => [i.version, i]));
    return releases.map((r) => {
      const inst = installedMap.get(r.version);
      const modes = [
        inst?.cpuInstalled && r.cpu.url ? "CPU" : null,
        inst?.cudaInstalled && r.cuda.url ? "CUDA" : null,
        inst?.vulkanInstalled && r.vulkan.url ? "VULKAN" : null,
        inst?.hipInstalled && r.hip.url ? "HIP" : null,
      ].filter(Boolean);
      return {
        value: `${r.version}:auto`,
        label: modes.length > 0 ? `${r.version} · ${modes.join("/")}` : r.version,
        icon: modes.length > 0 ? "Check" : undefined,
      };
    });
  }, [catalog, releases]);

  const currentVersion = `${rt.runtimeVersion ?? releaseVersion}:${rt.mode}`;

  return (
    <div className="mx-auto max-w-[720px] space-y-3">
      <SectionCard title={t("runtime.managedTitle")}>
        <div className="space-y-3">
          <Field label={t("runtime.contentDirectory")}>
            <TextInput mono value={draft.contentDirectory} onChange={(v) => patch({ contentDirectory: v })} />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label={t("runtime.installedRuntime")} hint={status?.running ? status.endpoint : undefined}>
              <Dropdown
                value={versionOptions.some((o) => o.value === currentVersion) ? currentVersion : (versionOptions[0]?.value ?? "")}
                onChange={(v) => {
                  const [version, mode] = v.split(":");
                  patch({
                    llamaRuntime: {
                      ...rt,
                      runtimeVersion: version,
                      mode: (["auto", "cpu", "cuda", "vulkan", "hip"].includes(mode) ? mode : "auto") as RuntimeMode,
                    },
                  });
                }}
                options={versionOptions}
              />
            </Field>
            <Field label={t("runtime.contextSize")}>
              <NumberInput
                value={rt.contextSize}
                min={1024}
                onChange={(v) => patch({ llamaRuntime: { ...rt, contextSize: v } })}
              />
            </Field>
          </div>
          <Field label={t("runtime.model")}>
            <Dropdown
              value={rt.modelPath}
              onChange={(v) => patch({ llamaRuntime: { ...rt, modelPath: v } })}
              placeholder={t("runtime.selectModel")}
              options={[
                ...(rt.modelPath && !models.some((m) => m.path === rt.modelPath)
                  ? [{ value: rt.modelPath, label: rt.modelPath.split(/[\\/]/).pop() ?? rt.modelPath }]
                  : []),
                ...models.map((m) => ({ value: m.path, label: m.name, icon: "HardDrive" })),
              ]}
            />
          </Field>
          <ToggleRow
            title={t("runtime.autoStartTitle")}
            description={t("runtime.autoStartDescription")}
            on={rt.autoStart}
            onChange={(v) => patch({ llamaRuntime: { ...rt, autoStart: v } })}
          />
          <div className="flex items-center gap-2">
            <Button
              icon={busy === "start" ? "Loader2" : status?.running ? "Square" : "Play"}
              spin={busy === "start"}
              variant={status?.running ? "solid" : "primary"}
              onClick={() => void startStop()}
            >
              {status?.running ? t("runtime.stop") : t("runtime.start")}
            </Button>
            <Button icon="RefreshCw" variant="solid" onClick={() => void refresh()}>
              {t("common.refresh")}
            </Button>
            <span className="ml-auto flex items-center gap-1.5 text-[11.5px] text-fg-subtle">
              <span className={cn("h-1.5 w-1.5 rounded-full", status?.running ? "bg-success pulse-ring" : "bg-ink-500")} />
              {status?.running ? status.endpoint || t("runtime.running") : t("runtime.stopped")}
            </span>
          </div>
        </div>
      </SectionCard>

      <SectionCard title={t("runtime.installerTitle")}>
        <div className="space-y-3">
          <div className="flex gap-2">
            <Dropdown
              value={releaseVersion}
              onChange={setReleaseVersion}
              className="flex-1"
              options={releases.map((r) => ({ value: r.version, label: r.version, icon: "Download" }))}
            />
            <Button icon="RefreshCw" variant="solid" onClick={() => void refresh()}>
              {t("runtime.browseReleases")}
            </Button>
          </div>
          <div className="flex flex-wrap gap-2">
            {(["cpu", "cuda", "vulkan", "hip"] as const).map((mode) => {
              const release = releases.find((r) => r.version === releaseVersion);
              const available =
                (mode === "cpu" && release?.cpu.url) ||
                (mode === "cuda" && release?.cuda.url) ||
                (mode === "vulkan" && release?.vulkan.url) ||
                (mode === "hip" && release?.hip.url);
              return (
                <Button
                  key={mode}
                  icon={busy === `runtime-${mode}` ? "Loader2" : "Download"}
                  spin={busy === `runtime-${mode}`}
                  variant="solid"
                  disabled={!available || Boolean(busy)}
                  onClick={() => void installRuntime(mode)}
                >
                  {t("runtime.installFor", { mode: mode.toUpperCase() })}
                </Button>
              );
            })}
          </div>
          {progress && busy?.startsWith("runtime-") && <ProgressBar progress={progress} />}
        </div>
      </SectionCard>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* api                                                                 */
/* ------------------------------------------------------------------ */

function ApiPanel({
  draft,
  patch,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
}) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<APIStatus | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [originsText, setOriginsText] = useState(draft.api.allowedOrigins.join(", "));

  useEffect(() => {
    void desktop.getAPIStatus().then(setStatus).catch(() => undefined);
  }, []);

  const api = draft.api;
  const isLoopback = api.bindAddress === "127.0.0.1" || api.bindAddress === "::1";
  const exposureWarning = api.enabled && (!isLoopback || api.authMode === "none");

  const rotateToken = async () => {
    try {
      const next = await desktop.rotateAPIToken();
      setToken(next);
      void desktop.getAPIStatus().then(setStatus).catch(() => undefined);
    } catch {
      /* surfaced by status message */
    }
  };

  const commitOrigins = (raw: string) => {
    setOriginsText(raw);
    const origins = raw
      .split(/[\n,]/)
      .map((o) => o.trim())
      .filter(Boolean);
    patch({ api: { ...api, allowedOrigins: origins } });
  };

  return (
    <div className="mx-auto max-w-[720px]">
      <SectionCard title={t("api.title")}>
        <div className="space-y-3">
          <ToggleRow
            title={t("api.enableTitle")}
            description={t("api.enableDescription")}
            on={api.enabled}
            onChange={(v) => patch({ api: { ...api, enabled: v } })}
          />
          <div className={cn("grid grid-cols-[1.4fr_0.6fr] gap-3", !api.enabled && "pointer-events-none opacity-40")}>
            <Field label={t("api.bindAddress")}>
              <TextInput mono value={api.bindAddress} onChange={(v) => patch({ api: { ...api, bindAddress: v } })} placeholder="127.0.0.1" />
            </Field>
            <Field label={t("api.port")}>
              <NumberInput value={api.port} min={1024} max={65535} onChange={(v) => patch({ api: { ...api, port: v } })} />
            </Field>
          </div>
          <div className={cn("flex items-end gap-2", !api.enabled && "pointer-events-none opacity-40")}>
            <Field label={t("api.authMode")} className="min-w-0 flex-1">
              <Dropdown
                value={api.authMode}
                onChange={(v) =>
                  patch({
                    api: {
                      ...api,
                      authMode: v as typeof api.authMode,
                      adminEnabled: v === "token" ? api.adminEnabled : false,
                    },
                  })
                }
                options={[
                  { value: "token", label: t("api.authToken"), icon: "KeyRound" },
                  { value: "none", label: t("api.authNone") },
                ]}
              />
            </Field>
            <Button icon="KeyRound" variant="solid" onClick={() => void rotateToken()} disabled={!api.enabled}>
              {status?.tokenConfigured ? t("api.rotateToken") : t("api.createToken")}
            </Button>
          </div>
          <Field label={t("api.allowedOrigins")}>
            <TextArea rows={2} value={originsText} onChange={commitOrigins} placeholder="https://dashboard.example.com" />
          </Field>
          {api.authMode === "token" && (
            <ToggleRow
              title={t("api.adminTitle")}
              description={t("api.adminDescription")}
              on={api.adminEnabled}
              onChange={(v) => patch({ api: { ...api, adminEnabled: v } })}
            />
          )}
          {exposureWarning && (
            <div className="flex items-center gap-2 rounded-lg border border-warning/30 bg-warning/10 px-3 py-2 text-[11.5px] text-warning-fg">
              <Icon name="AlertTriangle" className="h-3.5 w-3.5 shrink-0" />
              {t("api.tlsWarning")}
            </div>
          )}
          <div className="flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2 text-[12px] text-fg-subtle">
            <span className={cn("h-2 w-2 rounded-full", status?.running ? "bg-success" : "bg-ink-500")} />
            {status?.running ? status.endpoint : (status?.message ?? t("api.disabled"))}
          </div>
        </div>
      </SectionCard>

      {token && (
        <Modal title={t("api.tokenTitle")} icon="KeyRound" onClose={() => setToken(null)} size="sm"
          footer={<ModalActions onCancel={() => setToken(null)} onConfirm={() => setToken(null)} confirmLabel={t("common.close")} />}>
          <p className="mb-2 text-[12px] text-fg-subtle">{t("api.tokenOnce")}</p>
          <div className="flex gap-2">
            <code className="min-w-0 flex-1 overflow-x-auto rounded-md border border-ink-700 bg-ink-950/60 px-2.5 py-2 font-mono text-[11px] break-all whitespace-pre-wrap text-fg">
              {token}
            </code>
            <Button icon="Copy" variant="solid" onClick={() => navigator.clipboard?.writeText(token)}>
              {t("common.copy")}
            </Button>
          </div>
        </Modal>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* twitch                                                              */
/* ------------------------------------------------------------------ */

function TwitchPanel({
  draft,
  patch,
  triggers,
  refreshTriggers,
  refreshSettings,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
  triggers: TriggerBinding[];
  refreshTriggers: () => Promise<void>;
  refreshSettings: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<TwitchStatus | null>(null);
  const [catalog, setCatalog] = useState<TwitchEventDescriptor[]>([]);
  const [auth, setAuth] = useState<TwitchDeviceAuthorization | null>(null);
  const [authLabel, setAuthLabel] = useState("");
  const [manualOpen, setManualOpen] = useState(false);
  const [manualLabel, setManualLabel] = useState("");
  const [manualToken, setManualToken] = useState("");

  const twitchTriggers = triggers.filter((tr) => tr.kind === "twitch");

  const refresh = useCallback(async () => {
    const [st, cat] = await Promise.all([
      desktop.getTwitchStatus().catch(() => null),
      desktop.listTwitchEventCatalog().catch(() => []),
      refreshTriggers(),
      refreshSettings(),
    ]);
    setStatus(st);
    setCatalog(cat);
  }, [refreshTriggers, refreshSettings]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  /* keep the panel live while it is open: device-code auth completes and
     identities land server-side asynchronously, and the EventSub socket
     connects seconds after trust/enable — a one-shot fetch goes stale */
  useEffect(() => {
    const timer = window.setInterval(() => void refresh(), PANEL_POLL_MS);
    return () => window.clearInterval(timer);
  }, [refresh]);

  const connectScopes = useMemo(
    () => [...new Set(["user:read:chat", "user:write:chat", ...catalog.flatMap((e) => e.requiredScopes)])],
    [catalog],
  );

  const startAuth = async (identity?: TwitchIdentity) => {
    try {
      await workspace_save(draft);
      const authorization = await desktop.startTwitchDeviceAuthorization({
        identityId: identity?.id,
        label: authLabel || identity?.label || t("twitch.identityLabelPlaceholder"),
        scopes: connectScopes,
      });
      setAuth(authorization);
    } catch {
      /* error surfaces via status */
    }
  };

  const cancelAuth = async () => {
    if (!auth) return;
    await desktop.cancelTwitchDeviceAuthorization(auth.id).catch(() => undefined);
    setAuth(null);
  };

  const addManual = async () => {
    if (!manualLabel.trim() || !manualToken.trim()) return;
    const request: TwitchManualIdentityRequest = { label: manualLabel.trim(), accessToken: manualToken.trim() };
    setManualOpen(false);
    setManualLabel("");
    setManualToken("");
    try {
      await desktop.addTwitchManualIdentity(request);
      await refresh();
    } catch {
      /* keep dialog state cleared; status shows failure */
    }
  };

  const removeIdentity = async (identity: TwitchIdentity) => {
    const ok = await ask({
      title: t("twitch.removeTitle"),
      description: t("twitch.removeDescription", { name: identity.label }),
      confirmLabel: t("twitch.remove"),
      danger: true,
    });
    if (!ok) return;
    await desktop.removeTwitchIdentity(identity.id).catch(() => undefined);
    await refresh();
  };

  const trustTrigger = async (binding: TriggerBinding) => {
    await desktop.trustTwitchTrigger(binding.id).catch(() => undefined);
    await refresh();
  };

  const toggleTrigger = async (binding: TriggerBinding, enabled: boolean) => {
    await desktop.setTwitchTriggerEnabled(binding.id, enabled).catch(() => undefined);
    await refresh();
  };

  const openVerification = async () => {
    if (!auth) return;
    try {
      await Browser.OpenURL(auth.verificationUri);
    } catch {
      /* outside Wails */
    }
  };

  return (
    <div className="mx-auto max-w-[720px] space-y-3">
      <SectionCard title={t("twitch.connection")}>
        <div className="space-y-3">
          <Field label={t("twitch.clientId")}>
            <TextInput mono value={draft.twitch.clientId} onChange={(v) => patch({ twitch: { ...draft.twitch, clientId: v } })} placeholder={t("twitch.clientIdPlaceholder")} />
          </Field>
          <div className="flex items-center justify-between rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
            <div className="min-w-0">
              <p className="text-[12.5px] font-medium text-fg">
                {status?.connected ? t("twitch.connected") : t("twitch.disconnected")}
              </p>
              <p className="truncate text-[11px] text-fg-faint">
                {status?.lastError || t("twitch.eventSubDescription", { count: status?.activeSubscriptions ?? 0 })}
              </p>
            </div>
            {status?.connected && <Icon name="Check" className="h-4 w-4 shrink-0 text-success-fg" />}
          </div>
        </div>
      </SectionCard>

      <SectionCard title={t("twitch.identities")}>
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <Button
              icon="Cable"
              variant="primary"
              disabled={!draft.twitch.clientId.trim()}
              onClick={() => {
                setAuthLabel("");
                void startAuth();
              }}
            >
              {t("twitch.connect")}
            </Button>
            <Button icon="KeyRound" variant="solid" onClick={() => setManualOpen(true)}>
              {t("twitch.manualToken")}
            </Button>
          </div>

          <Field label={t("twitch.defaultBotIdentity")}>
            <Dropdown
              value={draft.twitch.defaultBotIdentityId ?? ""}
              onChange={(v) => patch({ twitch: { ...draft.twitch, defaultBotIdentityId: v || undefined } })}
              placeholder={t("twitch.defaultBotIdentityPlaceholder")}
              options={[
                { value: "", label: t("twitch.defaultBotIdentityPlaceholder") },
                ...draft.twitch.identities
                  .filter((identity) => identity.status === "connected")
                  .map((identity) => ({ value: identity.id, label: identity.label, icon: "Bot" })),
              ]}
            />
          </Field>

          {draft.twitch.identities.length === 0 ? (
            <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
              {t("twitch.noIdentities")}
            </p>
          ) : (
            draft.twitch.identities.map((identity) => (
              <div key={identity.id} className="flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[12.5px] font-medium text-fg">{identity.label}</p>
                  <p className="truncate text-[11px] text-fg-faint">
                    {identity.login} · {identity.scopes.length > 0 ? identity.scopes.join(", ") : t("twitch.noScopes")}
                  </p>
                </div>
                {identity.status !== "connected" && (
                  <span className="shrink-0 rounded bg-warning/15 px-2 py-1 text-[10.5px] text-warning-fg">
                    {t("twitch.reconnectRequired")}
                  </span>
                )}
                {identity.status !== "connected" && (
                  <Button variant="solid" onClick={() => void startAuth(identity)}>
                    {t("twitch.reconnect")}
                  </Button>
                )}
                <Button icon="Trash2" variant="solid" onClick={() => void removeIdentity(identity)}>
                  {t("common.delete")}
                </Button>
              </div>
            ))
          )}
        </div>
      </SectionCard>

      <SectionCard title={t("twitch.triggers")}>
        <p className="mb-3 text-[11.5px] leading-relaxed text-fg-faint">{t("twitch.triggersHelp")}</p>
        {twitchTriggers.length === 0 ? (
          <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
            {t("twitch.noTriggers")}
          </p>
        ) : (
          twitchTriggers.map((binding) => (
            <div key={binding.id} className="flex items-center gap-3 border-b border-seam/70 py-2 last:border-b-0">
              <span className="min-w-0 flex-1 truncate text-[12.5px] font-medium text-fg">{binding.label}</span>
              {!binding.trusted ? (
                <Button icon="ShieldCheck" variant="solid" onClick={() => void trustTrigger(binding)}>
                  {t("schedules.trust")}
                </Button>
              ) : (
                <Toggle on={binding.enabled} onChange={(v) => void toggleTrigger(binding, v)} />
              )}
            </div>
          ))
        )}
      </SectionCard>

      {/* device-code flow */}
      {auth && (
        <Modal
          title={t("twitch.connectTitle")}
          icon="ShieldCheck"
          onClose={() => void cancelAuth()}
          footer={
            <div className="ml-auto flex items-center gap-2">
              <Button variant="ghost" onClick={() => void cancelAuth()}>
                {t("common.cancel")}
              </Button>
              <Button icon="ExternalLink" variant="primary" onClick={() => void openVerification()}>
                {t("twitch.openTwitch")}
              </Button>
            </div>
          }
        >
          <div className="space-y-3 text-center">
            <p className="text-[12.5px] text-fg-subtle">{t("twitch.openVerification", { url: auth.verificationUri })}</p>
            <code className="block rounded-xl border border-ink-650 bg-ink-950/70 px-4 py-4 font-mono text-[26px] tracking-[0.35em] text-fg select-all">
              {auth.userCode}
            </code>
            <p className="text-[11px] text-fg-faint">{t("twitch.expiresAt", { time: formatDateTime(auth.expiresAt) })}</p>
          </div>
        </Modal>
      )}

      {/* manual token */}
      {manualOpen && (
        <Modal
          title={t("twitch.manualTitle")}
          icon="KeyRound"
          onClose={() => setManualOpen(false)}
          footer={
            <ModalActions
              onCancel={() => setManualOpen(false)}
              onConfirm={() => void addManual()}
              confirmLabel={t("common.save")}
              disabled={!manualLabel.trim() || !manualToken.trim()}
            />
          }
        >
          <div className="space-y-3">
            <p className="text-[12px] leading-relaxed text-fg-subtle">{t("twitch.manualDescription")}</p>
            <Field label={t("twitch.identityLabel")}>
              <TextInput autoFocus value={manualLabel} onChange={setManualLabel} placeholder={t("twitch.identityLabelPlaceholder")} />
            </Field>
            <Field label={t("twitch.accessToken")}>
              <input
                type="password"
                autoComplete="off"
                value={manualToken}
                onChange={(e) => setManualToken(e.target.value)}
                className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 font-mono text-[12px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
              />
            </Field>
          </div>
        </Modal>
      )}
    </div>
  );
}

async function workspace_save(settings: Settings): Promise<void> {
  await desktop.saveSettings(settings);
}

/* ------------------------------------------------------------------ */
/* discord                                                             */
/* ------------------------------------------------------------------ */

function DiscordPanel({
  draft,
  patch,
  triggers,
  refreshTriggers,
  refreshSettings,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
  triggers: TriggerBinding[];
  refreshTriggers: () => Promise<void>;
  refreshSettings: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<DiscordStatus | null>(null);
  const [catalog, setCatalog] = useState<DiscordEventDescriptor[]>([]);
  const [addOpen, setAddOpen] = useState(false);
  const [addLabel, setAddLabel] = useState("");
  const [addToken, setAddToken] = useState("");
  const [addError, setAddError] = useState("");

  const discordTriggers = triggers.filter((tr) => tr.kind === "discord");

  const refresh = useCallback(async () => {
    const [st, cat] = await Promise.all([
      desktop.getDiscordStatus().catch(() => null),
      desktop.listDiscordEventCatalog().catch(() => [] as DiscordEventDescriptor[]),
      refreshTriggers(),
      refreshSettings(),
    ]);
    setStatus(st);
    setCatalog(cat);
  }, [refreshTriggers, refreshSettings]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  /* keep the panel live while it is open: the gateway session opens
     asynchronously after add-bot/trust/enable, so a one-shot fetch would
     keep showing the previous connection state */
  useEffect(() => {
    const timer = window.setInterval(() => void refresh(), PANEL_POLL_MS);
    return () => window.clearInterval(timer);
  }, [refresh]);

  const needsPrivilegedIntents = useMemo(
    () => catalog.some((descriptor) => descriptor.privileged),
    [catalog],
  );

  const addBot = async () => {
    if (!addToken.trim()) return;
    try {
      await desktop.addDiscordManualIdentity({ label: addLabel.trim(), token: addToken.trim() });
      setAddOpen(false);
      setAddLabel("");
      setAddToken("");
      setAddError("");
      await refresh();
    } catch (error) {
      setAddError(String((error as { message?: string })?.message ?? error));
    }
  };

  const removeIdentity = async (identity: DiscordIdentity) => {
    const ok = await ask({
      title: t("discord.removeTitle"),
      description: t("discord.removeDescription", { name: identity.label }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    await desktop.removeDiscordIdentity(identity.id).catch(() => undefined);
    await refresh();
  };

  const trustTrigger = async (binding: TriggerBinding) => {
    await desktop.trustDiscordTrigger(binding.id).catch(() => undefined);
    await refreshTriggers();
    await refresh();
  };

  const toggleTrigger = async (binding: TriggerBinding, enabled: boolean) => {
    await desktop.setDiscordTriggerEnabled(binding.id, enabled).catch(() => undefined);
    await refreshTriggers();
    await refresh();
  };

  return (
    <div className="mx-auto max-w-[720px] space-y-3">
      <SectionCard title={t("discord.connection")}>
        <div className="space-y-3">
          <div className="flex items-center justify-between rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
            <div className="min-w-0">
              <p className="text-[12.5px] font-medium text-fg">
                {status?.connected ? t("discord.connected") : t("discord.disconnected")}
              </p>
              <p className="truncate text-[11px] text-fg-faint">
                {status?.lastError || t("discord.gatewayDescription", { count: status?.activeSubscriptions ?? 0 })}
              </p>
            </div>
            {status?.connected && <Icon name="Check" className="h-4 w-4 shrink-0 text-success-fg" />}
          </div>
          {needsPrivilegedIntents && (
            <p className="rounded-lg border border-warning/30 bg-warning/10 px-3 py-2.5 text-[11.5px] leading-relaxed text-warning-fg">
              {t("discord.intentsWarning")}
            </p>
          )}
        </div>
      </SectionCard>

      <SectionCard title={t("discord.identities")}>
        <div className="space-y-3">
          <Button icon="Cable" variant="primary" onClick={() => setAddOpen(true)}>
            {t("discord.addBot")}
          </Button>

          <Field label={t("discord.defaultBotIdentity")}>
            <Dropdown
              value={draft.discord.defaultBotIdentityId ?? ""}
              onChange={(v) => patch({ discord: { ...draft.discord, defaultBotIdentityId: v || undefined } })}
              placeholder={t("discord.defaultBotIdentityPlaceholder")}
              options={[
                { value: "", label: t("discord.defaultBotIdentityPlaceholder") },
                ...draft.discord.identities
                  .filter((identity) => identity.status === "connected")
                  .map((identity) => ({ value: identity.id, label: identity.label, icon: "Bot" })),
              ]}
            />
          </Field>

          {draft.discord.identities.length === 0 ? (
            <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
              {t("discord.noIdentities")}
            </p>
          ) : (
            draft.discord.identities.map((identity) => (
              <div key={identity.id} className="flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[12.5px] font-medium text-fg">{identity.label}</p>
                  <p className="truncate text-[11px] text-fg-faint">@{identity.username}</p>
                </div>
                {identity.status !== "connected" && (
                  <span className="shrink-0 rounded bg-warning/15 px-2 py-1 text-[10.5px] text-warning-fg">
                    {t("discord.invalidIdentity")}
                  </span>
                )}
                <Button icon="Trash2" variant="solid" onClick={() => void removeIdentity(identity)}>
                  {t("common.delete")}
                </Button>
              </div>
            ))
          )}
        </div>
      </SectionCard>

      <SectionCard title={t("discord.triggers")}>
        <p className="mb-3 text-[11.5px] leading-relaxed text-fg-faint">{t("discord.triggersHelp")}</p>
        {discordTriggers.length === 0 ? (
          <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
            {t("discord.noTriggers")}
          </p>
        ) : (
          discordTriggers.map((binding) => (
            <div key={binding.id} className="flex items-center gap-3 border-b border-seam/70 py-2 last:border-b-0">
              <span className="min-w-0 flex-1 truncate text-[12.5px] font-medium text-fg">{binding.label}</span>
              {!binding.trusted ? (
                <Button icon="ShieldCheck" variant="solid" onClick={() => void trustTrigger(binding)}>
                  {t("schedules.trust")}
                </Button>
              ) : (
                <Toggle on={binding.enabled} onChange={(v) => void toggleTrigger(binding, v)} />
              )}
            </div>
          ))
        )}
      </SectionCard>

      {addOpen && (
        <Modal
          title={t("discord.addTitle")}
          icon="KeyRound"
          onClose={() => setAddOpen(false)}
          footer={
            <ModalActions
              onCancel={() => setAddOpen(false)}
              onConfirm={() => void addBot()}
              confirmLabel={t("common.save")}
              disabled={!addToken.trim()}
            />
          }
        >
          <div className="space-y-3">
            <p className="text-[12px] leading-relaxed text-fg-subtle">{t("discord.tokenDescription")}</p>
            <Field label={t("discord.identityLabel")}>
              <TextInput autoFocus value={addLabel} onChange={setAddLabel} placeholder={t("discord.identityLabelPlaceholder")} />
            </Field>
            <Field label={t("discord.botToken")}>
              <input
                type="password"
                autoComplete="off"
                value={addToken}
                onChange={(e) => setAddToken(e.target.value)}
                className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 font-mono text-[12px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
              />
            </Field>
            {addError && <p className="text-[11.5px] text-danger-fg">{addError}</p>}
          </div>
        </Modal>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* telegram                                                            */
/* ------------------------------------------------------------------ */

function TelegramPanel({
  draft,
  patch,
  triggers,
  refreshTriggers,
  refreshSettings,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
  triggers: TriggerBinding[];
  refreshTriggers: () => Promise<void>;
  refreshSettings: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<TelegramStatus | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [addLabel, setAddLabel] = useState("");
  const [addToken, setAddToken] = useState("");
  const [addError, setAddError] = useState("");

  const telegramTriggers = triggers.filter((tr) => tr.kind === "telegram");

  const refresh = useCallback(async () => {
    const st = await desktop.getTelegramStatus().catch(() => null);
    setStatus(st);
    await Promise.all([refreshTriggers(), refreshSettings()]);
  }, [refreshTriggers, refreshSettings]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  /* keep the panel live while it is open: the long-poll loop (re)starts
     asynchronously after add-bot/trust/enable, so a one-shot fetch would
     keep showing the previous connection state */
  useEffect(() => {
    const timer = window.setInterval(() => void refresh(), PANEL_POLL_MS);
    return () => window.clearInterval(timer);
  }, [refresh]);

  const addBot = async () => {
    if (!addToken.trim()) return;
    try {
      await desktop.addTelegramManualIdentity({ label: addLabel.trim(), token: addToken.trim() });
      setAddOpen(false);
      setAddLabel("");
      setAddToken("");
      setAddError("");
      await refresh();
    } catch (error) {
      setAddError(String((error as { message?: string })?.message ?? error));
    }
  };

  const removeIdentity = async (identity: TelegramIdentity) => {
    const ok = await ask({
      title: t("telegram.removeTitle"),
      description: t("telegram.removeDescription", { name: identity.label }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    await desktop.removeTelegramIdentity(identity.id).catch(() => undefined);
    await refresh();
  };

  const trustTrigger = async (binding: TriggerBinding) => {
    await desktop.trustTelegramTrigger(binding.id).catch(() => undefined);
    await refreshTriggers();
    await refresh();
  };

  const toggleTrigger = async (binding: TriggerBinding, enabled: boolean) => {
    await desktop.setTelegramTriggerEnabled(binding.id, enabled).catch(() => undefined);
    await refreshTriggers();
    await refresh();
  };

  return (
    <div className="mx-auto max-w-[720px] space-y-3">
      <SectionCard title={t("telegram.connection")}>
        <div className="space-y-3">
          <div className="flex items-center justify-between rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
            <div className="min-w-0">
              <p className="text-[12.5px] font-medium text-fg">
                {status?.connected ? t("telegram.connected") : t("telegram.disconnected")}
              </p>
              <p className="truncate text-[11px] text-fg-faint">
                {status?.lastError || t("telegram.pollingDescription", { count: status?.activeSubscriptions ?? 0 })}
              </p>
            </div>
            {status?.connected && <Icon name="Check" className="h-4 w-4 shrink-0 text-success-fg" />}
          </div>
          <p className="rounded-lg border border-ink-700 bg-ink-900/40 px-3 py-2.5 text-[11.5px] leading-relaxed text-fg-faint">
            {t("telegram.privacyModeHint")}
          </p>
        </div>
      </SectionCard>

      <SectionCard title={t("telegram.identities")}>
        <div className="space-y-3">
          <Button icon="Cable" variant="primary" onClick={() => setAddOpen(true)}>
            {t("telegram.addBot")}
          </Button>

          <Field label={t("telegram.defaultBotIdentity")}>
            <Dropdown
              value={draft.telegram.defaultBotIdentityId ?? ""}
              onChange={(v) => patch({ telegram: { ...draft.telegram, defaultBotIdentityId: v || undefined } })}
              placeholder={t("telegram.defaultBotIdentityPlaceholder")}
              options={[
                { value: "", label: t("telegram.defaultBotIdentityPlaceholder") },
                ...draft.telegram.identities
                  .filter((identity) => identity.status === "connected")
                  .map((identity) => ({ value: identity.id, label: identity.label, icon: "Bot" })),
              ]}
            />
          </Field>

          {draft.telegram.identities.length === 0 ? (
            <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
              {t("telegram.noIdentities")}
            </p>
          ) : (
            draft.telegram.identities.map((identity) => (
              <div key={identity.id} className="flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[12.5px] font-medium text-fg">{identity.label}</p>
                  <p className="truncate text-[11px] text-fg-faint">@{identity.username}</p>
                </div>
                {identity.status !== "connected" && (
                  <span className="shrink-0 rounded bg-warning/15 px-2 py-1 text-[10.5px] text-warning-fg">
                    {t("telegram.invalidIdentity")}
                  </span>
                )}
                <Button icon="Trash2" variant="solid" onClick={() => void removeIdentity(identity)}>
                  {t("common.delete")}
                </Button>
              </div>
            ))
          )}
        </div>
      </SectionCard>

      <SectionCard title={t("telegram.triggers")}>
        <p className="mb-3 text-[11.5px] leading-relaxed text-fg-faint">{t("telegram.triggersHelp")}</p>
        {telegramTriggers.length === 0 ? (
          <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
            {t("telegram.noTriggers")}
          </p>
        ) : (
          telegramTriggers.map((binding) => (
            <div key={binding.id} className="flex items-center gap-3 border-b border-seam/70 py-2 last:border-b-0">
              <span className="min-w-0 flex-1 truncate text-[12.5px] font-medium text-fg">{binding.label}</span>
              {!binding.trusted ? (
                <Button icon="ShieldCheck" variant="solid" onClick={() => void trustTrigger(binding)}>
                  {t("schedules.trust")}
                </Button>
              ) : (
                <Toggle on={binding.enabled} onChange={(v) => void toggleTrigger(binding, v)} />
              )}
            </div>
          ))
        )}
      </SectionCard>

      {addOpen && (
        <Modal
          title={t("telegram.addTitle")}
          icon="KeyRound"
          onClose={() => setAddOpen(false)}
          footer={
            <ModalActions
              onCancel={() => setAddOpen(false)}
              onConfirm={() => void addBot()}
              confirmLabel={t("common.save")}
              disabled={!addToken.trim()}
            />
          }
        >
          <div className="space-y-3">
            <p className="text-[12px] leading-relaxed text-fg-subtle">{t("telegram.tokenDescription")}</p>
            <Field label={t("telegram.identityLabel")}>
              <TextInput autoFocus value={addLabel} onChange={setAddLabel} placeholder={t("telegram.identityLabelPlaceholder")} />
            </Field>
            <Field label={t("telegram.botToken")}>
              <input
                type="password"
                autoComplete="off"
                value={addToken}
                onChange={(e) => setAddToken(e.target.value)}
                className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 font-mono text-[12px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
              />
            </Field>
            {addError && <p className="text-[11.5px] text-danger-fg">{addError}</p>}
          </div>
        </Modal>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* execution                                                           */
/* ------------------------------------------------------------------ */

function ExecutionPanel({ draft, patch }: { draft: Settings; patch: (p: Partial<Settings>) => void }) {
  const { t } = useTranslation();
  return (
    <div className="mx-auto max-w-[640px] space-y-3">
      <SectionCard title={t("settings.execution")}>
        <div className="grid grid-cols-2 gap-3">
          <Field label={t("execution.retentionDays")} hint={t("execution.retentionDaysHelp")}>
            <NumberInput value={draft.retentionDays} min={1} onChange={(v) => patch({ retentionDays: v })} />
          </Field>
          <Field label={t("execution.webhookPort")}>
            <NumberInput value={draft.webhookPort} min={1024} max={65535} onChange={(v) => patch({ webhookPort: v })} />
          </Field>
          <Field label={t("execution.maxConcurrentRuns")} hint={t("execution.maxRunsHelp")}>
            <NumberInput value={draft.maxConcurrentRuns} min={1} max={16} onChange={(v) => patch({ maxConcurrentRuns: v })} />
          </Field>
          <Field label={t("execution.maxConcurrentLLM")} hint={t("execution.maxLLMHelp")}>
            <NumberInput value={draft.maxConcurrentLLMRuns} min={1} max={8} onChange={(v) => patch({ maxConcurrentLLMRuns: v })} />
          </Field>
        </div>
      </SectionCard>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* metrics settings                                                    */
/* ------------------------------------------------------------------ */

function MetricsPanel({ draft, patch }: { draft: Settings; patch: (p: Partial<Settings>) => void }) {
  const { t } = useTranslation();

  const clearMetrics = async () => {
    const ok = await ask({
      title: t("metrics.clearTitle"),
      description: t("metrics.clearDescription"),
      confirmLabel: t("metrics.clearConfirm"),
      danger: true,
    });
    if (!ok) return;
    await desktop.clearMetrics().catch(() => undefined);
  };

  const updateRate = (index: number, key: "model" | "inputUsdPerMillion" | "outputUsdPerMillion", value: string | number) => {
    patch({
      metrics: {
        ...draft.metrics,
        priceRates: draft.metrics.priceRates.map((rate, i) => (i === index ? { ...rate, [key]: value } : rate)),
      },
    });
  };

  return (
    <div className="mx-auto max-w-[640px] space-y-3">
      <SectionCard title={t("settings.metrics")}>
        <div className="grid grid-cols-2 gap-3">
          <Field label={t("metrics.detailRetention")} hint={t("metrics.detailRetentionHelp")}>
            <NumberInput
              value={draft.metrics.detailRetentionDays}
              min={1}
              max={365}
              onChange={(v) => patch({ metrics: { ...draft.metrics, detailRetentionDays: v } })}
            />
          </Field>
          <Field label={t("metrics.rollupRetention")} hint={t("metrics.rollupRetentionHelp")}>
            <NumberInput
              value={draft.metrics.rollupRetentionDays}
              min={30}
              max={3650}
              onChange={(v) => patch({ metrics: { ...draft.metrics, rollupRetentionDays: v } })}
            />
          </Field>
          <Field label={t("metrics.sampleInterval")} hint={t("metrics.sampleIntervalHelp")}>
            <NumberInput
              value={draft.metrics.sampleIntervalSeconds}
              min={10}
              max={300}
              onChange={(v) => patch({ metrics: { ...draft.metrics, sampleIntervalSeconds: v } })}
            />
          </Field>
        </div>
        <div className="mt-3">
          <Button icon="Trash2" variant="solid" onClick={() => void clearMetrics()}>
            {t("metrics.clearMetrics")}
          </Button>
        </div>
      </SectionCard>

      <SectionCard
        title={t("metrics.priceRates")}
        action={
          <Button
            icon="Plus"
            onClick={() =>
              patch({
                metrics: {
                  ...draft.metrics,
                  priceRates: [
                    ...draft.metrics.priceRates,
                    {
                      providerId: draft.providers[0]?.id ?? "",
                      model: draft.providers[0]?.model ?? "",
                      inputUsdPerMillion: 0,
                      outputUsdPerMillion: 0,
                    },
                  ],
                },
              })
            }
          >
            {t("common.add")}
          </Button>
        }
      >
        {draft.metrics.priceRates.length === 0 ? (
          <p className="text-[12px] text-fg-faint">{t("metrics.noPriceRates")}</p>
        ) : (
          <div className="space-y-2">
            {draft.metrics.priceRates.map((rate, index) => (
              <div key={index} className="grid grid-cols-[minmax(0,1fr)_92px_92px_28px] items-center gap-2">
                <TextInput
                  mono
                  size="sm"
                  value={rate.model}
                  onChange={(v) => updateRate(index, "model", v)}
                  placeholder={t("metrics.rateModel")}
                />
                <input
                  type="number"
                  step="0.01"
                  value={rate.inputUsdPerMillion}
                  onChange={(e) => updateRate(index, "inputUsdPerMillion", Number(e.target.value))}
                  aria-label={t("metrics.rateInput")}
                  className="h-7 rounded-md border border-ink-700 bg-ink-850 px-2 text-[12px] text-fg focus:border-ink-500 focus:outline-none"
                />
                <input
                  type="number"
                  step="0.01"
                  value={rate.outputUsdPerMillion}
                  onChange={(e) => updateRate(index, "outputUsdPerMillion", Number(e.target.value))}
                  aria-label={t("metrics.rateOutput")}
                  className="h-7 rounded-md border border-ink-700 bg-ink-850 px-2 text-[12px] text-fg focus:border-ink-500 focus:outline-none"
                />
                <button
                  onClick={() =>
                    patch({
                      metrics: {
                        ...draft.metrics,
                        priceRates: draft.metrics.priceRates.filter((_, i) => i !== index),
                      },
                    })
                  }
                  aria-label={t("common.delete")}
                  className="grid h-7 place-items-center rounded-md text-fg-faint hover:bg-danger/15 hover:text-danger-fg"
                >
                  <Icon name="Trash2" className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}
      </SectionCard>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* extensions                                                          */
/* ------------------------------------------------------------------ */

function ExtensionsPanel({ draft, patch }: { draft: Settings; patch: (p: Partial<Settings>) => void }) {
  const { t } = useTranslation();
  const [plugins, setPlugins] = useState<PluginStatus[] | null>(null);
  const [busy, setBusy] = useState(false);

  const load = async () => {
    setPlugins(await desktop.listPlugins().catch(() => []));
  };

  useEffect(() => {
    void load();
  }, []);

  const rediscover = async () => {
    setBusy(true);
    setPlugins(await desktop.rediscoverPlugins().catch(() => plugins ?? []));
    setBusy(false);
  };

  return (
    <div className="mx-auto max-w-[720px]">
      <SectionCard
        title={t("settings.extensions")}
        action={
          <Button icon={busy ? "Loader2" : "RefreshCw"} spin={busy} variant="solid" onClick={() => void rediscover()}>
            {t("extensions.rediscover")}
          </Button>
        }
      >
        <div className="space-y-3">
          <Field label={t("extensions.pluginDirectory")}>
            <TextInput mono value={draft.pluginDirectory} onChange={(v) => patch({ pluginDirectory: v })} />
          </Field>
          {!plugins || plugins.length === 0 ? (
            <p className="rounded-lg border border-ink-700 bg-ink-900/50 px-3 py-3 text-[12px] text-fg-faint">
              {t("extensions.noneFound")}
            </p>
          ) : (
            plugins.map((plugin) => (
              <div key={plugin.id} className="flex items-center gap-3 rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
                <span className={cn("h-2 w-2 shrink-0 rounded-full", plugin.healthy ? "bg-success" : "bg-danger")} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[12.5px] font-medium text-fg">
                    {plugin.name} <span className="font-mono text-[10.5px] text-fg-faint">v{plugin.version}</span>
                  </p>
                  <p className="truncate text-[11px] text-fg-faint">
                    {plugin.healthy
                      ? t("extensions.nodes", { count: plugin.nodeCount })
                      : (plugin.error ?? t("extensions.unavailable"))}
                  </p>
                </div>
              </div>
            ))
          )}
        </div>
      </SectionCard>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* secrets                                                             */
/* ------------------------------------------------------------------ */

function SecretsPanel() {
  const { t } = useTranslation();
  const [secrets, setSecrets] = useState<SecretMetadata[]>([]);
  const [name, setName] = useState("");
  const [value, setValue] = useState("");
  const [busyName, setBusyName] = useState<string | null>(null);

  const load = async () => {
    setSecrets(await desktop.listSecrets().catch(() => []));
  };

  useEffect(() => {
    void load();
  }, []);

  const save = async () => {
    if (!name.trim() || !value.trim()) return;
    setBusyName(name.trim());
    await desktop.saveSecret(name.trim(), value).catch(() => undefined);
    setValue("");
    setName("");
    await load();
    setBusyName(null);
  };

  const remove = async (secretName: string) => {
    const ok = await ask({
      title: t("secrets.deleteTitle"),
      description: t("secrets.deleteDescription", { name: secretName }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    setBusyName(secretName);
    await desktop.deleteSecret(secretName).catch(() => undefined);
    await load();
    setBusyName(null);
  };

  return (
    <div className="mx-auto max-w-[720px]">
      <SectionCard title={t("settings.secrets")}>
        <div className="space-y-3">
          <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_auto] items-end gap-2">
            <Field label={t("secrets.name")}>
              <TextInput mono value={name} onChange={setName} placeholder={t("secrets.namePlaceholder")} />
            </Field>
            <Field label={t("secrets.value")}>
              <input
                type="password"
                autoComplete="new-password"
                value={value}
                onChange={(e) => setValue(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && void save()}
                className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
              />
            </Field>
            <Button
              icon={busyName ? "Loader2" : "Save"}
              spin={Boolean(busyName)}
              variant="primary"
              onClick={() => void save()}
              disabled={!name.trim() || !value.trim()}
            >
              {t("secrets.addSecret")}
            </Button>
          </div>

          <p className="text-[11px] leading-relaxed text-fg-faint">{t("secrets.vaultNote")}</p>

          {secrets.length > 0 && (
            <div className="overflow-hidden rounded-xl border border-ink-700/80">
              {secrets.map((s, i) => (
                <div key={s.name} className={cn("flex items-center gap-3 px-3 py-2.5", i > 0 && "border-t border-seam")}>
                  <Icon name="KeyRound" className="h-3.5 w-3.5 shrink-0 text-warning-fg/80" />
                  <span className="min-w-0 flex-1 truncate font-mono text-[12px] text-fg">{s.name}</span>
                  <span className="text-[11px] text-fg-faint">{formatDateTime(s.updatedAt)}</span>
                  <button
                    onClick={() => void remove(s.name)}
                    aria-label={`${t("common.delete")} ${s.name}`}
                    className="grid h-6 w-6 place-items-center rounded text-fg-faint transition hover:bg-danger/15 hover:text-danger-fg"
                  >
                    <Icon name="Trash2" className="h-3.5 w-3.5" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </SectionCard>
    </div>
  );
}


