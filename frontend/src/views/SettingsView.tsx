import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Browser } from "@wailsio/runtime";
import i18n from "@/i18n";
import { desktop } from "@/lib/bridge";
import type {
  APIStatus,
  GenerationParameters,
  InstallProgress,
  InstalledLlamaRuntime,
  LlamaRuntimeCatalogStatus,
  LlamaRuntimeRelease,
  LlamaRuntimeReleaseList,
  LlamaRuntimeSettings,
  LlamaRuntimeStatus,
  LocalModel,
  ModelConfig,
  ModelDetail,
  ModelSearchRequest,
  ModelSearchResult,
  PluginStatus,
  ProviderConfig,
  RuntimeMode,
  SecretMetadata,
  Settings,
} from "@/lib/types";
import { formatBytes, formatCompact, formatDateTime } from "@/lib/format";
import type { Workspace } from "@/features/workspace/useWorkspace";
import { ask } from "@/stores/confirmation";
import { useThemeStore } from "@/stores/theme";
import { Card, ViewShell, EmptyState } from "../components/ViewShell";
import { Button, Toggle } from "../components/ui";
import { Icon } from "../components/icons";
import { Dropdown, type DropdownOption } from "../components/Dropdown";
import { MarkdownRenderer } from "../components/MarkdownRenderer";
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
  { id: "execution", labelKey: "settings.execution", icon: "Play" },
  { id: "metrics", labelKey: "settings.metrics", icon: "Activity" },
  { id: "extensions", labelKey: "settings.extensions", icon: "Sparkles" },
  { id: "secrets", labelKey: "settings.secrets", icon: "KeyRound" },
] as const;

type SectionId = (typeof SECTIONS)[number]["id"];

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
export function mergeIdentitySlice<S extends IdentitySlice<{ id: string }>>(draft: S, backend: S): S {
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

/** Normalises a loaded Settings object before it enters the editor draft.
 *  Shared with the Integrations view, which edits the same draft shape. */
export function normalizeSettings(input: Settings): Settings {
  const providers = input.providers.length > 0 ? input.providers : [defaultProvider("ollama")];
  const defaultProviderId =
    providers.some((p) => p.id === input.defaultProviderId) ? input.defaultProviderId : (providers.find((p) => p.enabled) ?? providers[0]).id;
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
    defaultProviderId,
    managedLlamaRemoved: input.managedLlamaRemoved ?? false,
  };
}

function defaultProvider(kind: ProviderConfig["kind"], existing: ProviderConfig[] = []): ProviderConfig {
  switch (kind) {
    case "ollama":
      return { id: uniqueProviderId("ollama-local", existing), name: "Ollama (local)", kind, baseUrl: "http://127.0.0.1:11434", model: "", models: [], enabled: true };
    case "anthropic":
      // No example URL here on purpose: the backend fills the official
      // endpoint on save, and an empty field must never look pre-validated.
      return { id: uniqueProviderId("anthropic", existing), name: "Anthropic", kind, baseUrl: "", model: "", models: [], enabled: true };
    case "llamacpp":
      return { id: "llama-managed", name: "Managed llama.cpp", kind, baseUrl: "", model: "", models: [], enabled: true };
    default:
      // No example URL here on purpose: an empty field plus its placeholder
      // must never look like configuration that was loaded from settings.
      return { id: uniqueProviderId("openai-compatible", existing), name: "OpenAI-compatible", kind, baseUrl: "", model: "", models: [], enabled: true };
  }
}

function uniqueProviderId(base: string, providers: ProviderConfig[]): string {
  const taken = new Set(providers.map((p) => p.id));
  if (!taken.has(base)) return base;
  for (let counter = 2; ; counter++) {
    const candidate = `${base}-${counter}`;
    if (!taken.has(candidate)) return candidate;
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
          {section === "models" && (
            <ModelsPanel draft={draft} patch={patch} notify={workspace.notify} refreshSettings={workspace.refreshSettings} />
          )}
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

/* Optional numeric parameter input: an empty field stays unset, so the
 * provider keeps its own default for that value. */
function OptionalNumberInput({
  value,
  onChange,
  min,
  max,
  step,
  placeholder,
}: {
  value: number | undefined;
  onChange: (v: number | undefined) => void;
  min?: number;
  max?: number;
  step?: number;
  placeholder?: string;
}) {
  const [text, setText] = useState(() => (value === undefined ? "" : String(value)));
  useEffect(() => {
    setText(value === undefined ? "" : String(value));
  }, [value]);
  const commit = (raw: string) => {
    const trimmed = raw.trim();
    if (trimmed === "") {
      if (value !== undefined) onChange(undefined);
      return;
    }
    const n = Number(trimmed);
    if (Number.isNaN(n)) return;
    const clamped = Math.min(max ?? Number.MAX_SAFE_INTEGER, Math.max(min ?? Number.MIN_SAFE_INTEGER, n));
    if (clamped !== value) onChange(clamped);
    if (String(clamped) !== text) setText(String(clamped));
  };
  return (
    <input
      type="number"
      inputMode="decimal"
      value={text}
      min={min}
      max={max}
      step={step}
      placeholder={placeholder}
      onChange={(e) => setText(e.target.value)}
      onBlur={(e) => commit(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === "Enter") commit((e.target as HTMLInputElement).value);
      }}
      className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
    />
  );
}

interface ParamField {
  key: keyof GenerationParameters;
  labelKey: "provider.temperature" | "provider.topP" | "provider.topK" | "provider.maxTokens" | "provider.contextSize";
  step: number;
  min: number;
  max?: number;
}

const PARAM_FIELDS: readonly ParamField[] = [
  { key: "temperature", labelKey: "provider.temperature", step: 0.1, min: 0, max: 2 },
  { key: "topP", labelKey: "provider.topP", step: 0.05, min: 0, max: 1 },
  { key: "topK", labelKey: "provider.topK", step: 1, min: 0 },
  { key: "maxTokens", labelKey: "provider.maxTokens", step: 1, min: 1 },
  { key: "contextSize", labelKey: "provider.contextSize", step: 1024, min: 1024 },
];

/** Counts configured values so collapsed sections can flag themselves. */
function paramCount(params: GenerationParameters | undefined): number {
  if (!params) return 0;
  return PARAM_FIELDS.reduce((n, field) => (params[field.key] !== undefined ? n + 1 : n), 0);
}

/** Editor for one generation-parameter set: empty fields stay unset and
 * inherit, so a partial override only sends what the user chose. */
function GenerationParamsEditor({
  value,
  onChange,
  inherited,
}: {
  value: GenerationParameters | undefined;
  onChange: (next: GenerationParameters | undefined) => void;
  /** values shown as placeholders because a parent level sets them */
  inherited?: GenerationParameters;
}) {
  const { t } = useTranslation();
  const set = (key: keyof GenerationParameters, v: number | undefined) => {
    const next: GenerationParameters = { ...(value ?? {}) };
    if (v === undefined) delete next[key];
    else next[key] = v;
    const remaining = (Object.keys(next) as (keyof GenerationParameters)[]).some((k) => next[k] !== undefined);
    onChange(remaining ? next : undefined);
  };
  return (
    <div className="grid grid-cols-3 gap-2">
      {PARAM_FIELDS.map((field) => (
        <Field key={field.key} label={t(field.labelKey)}>
          <OptionalNumberInput
            value={value?.[field.key]}
            onChange={(v) => set(field.key, v)}
            min={field.min}
            max={field.max}
            step={field.step}
            placeholder={
              inherited?.[field.key] !== undefined
                ? t("provider.paramInherited", { value: String(inherited[field.key]) })
                : t("provider.paramUnset")
            }
          />
        </Field>
      ))}
    </div>
  );
}

/** Collapsible sub-section used by provider parameters and the model list:
 * a header row toggles the body, a badge can flag configured content, and an
 * optional action stays reachable while collapsed. */
function CollapsibleBlock({
  title,
  hint,
  badge,
  defaultOpen = false,
  action,
  children,
}: {
  title: string;
  hint?: string;
  badge?: string;
  defaultOpen?: boolean;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="rounded-md border border-ink-700/70 bg-ink-900/50">
      <div className="flex items-center gap-2 px-3 py-2">
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          aria-expanded={open}
          className="flex min-w-0 flex-1 items-center gap-2 text-left transition hover:bg-ink-850/60"
        >
          <Icon name={open ? "ChevronDown" : "ChevronRight"} className="h-3.5 w-3.5 shrink-0 text-fg-faint" />
          <span className="shrink-0 text-[11px] font-semibold tracking-wide text-fg-subtle uppercase">{title}</span>
          {badge && (
            <span className="shrink-0 rounded border border-ink-700 bg-ink-900 px-1.5 py-px text-[10px] text-fg-faint">{badge}</span>
          )}
          {hint && !open && <span className="min-w-0 flex-1 truncate text-[10.5px] text-fg-faint">{hint}</span>}
        </button>
        {action && <div className="ml-auto shrink-0">{action}</div>}
      </div>
      {open && <div className="space-y-2 border-t border-seam px-3 py-2.5">{children}</div>}
    </div>
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
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [discovering, setDiscovering] = useState<string | null>(null);

  useEffect(() => {
    desktop
      .listSecrets()
      .then((list) => setSecrets(list.map((s) => s.name)))
      .catch(() => notify(t("settings.secretsLoadFailed"), "AlertTriangle"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const providers = draft.providers;
  /* No provider is expanded until one is picked: a long configured list stays
   * compact and the panel opens collapsed, matching the provider card header
   * click used to expand it. */
  const selected = providers.find((p) => p.id === selectedId) ?? null;
  /* Exactly one managed llama.cpp provider can exist: the backend normalizer
   * rejects a second entry, so the add menu stops offering it once present. */
  const canAddManaged = !providers.some((p) => p.kind === "llamacpp");

  const updateProviders = (next: ProviderConfig[]) => patch({ providers: next });

  const patchProvider = (id: string, changes: Partial<ProviderConfig>) =>
    updateProviders(providers.map((p) => (p.id === id ? { ...p, ...changes } : p)));

  const addProvider = (kind: ProviderConfig["kind"]) => {
    const provider = defaultProvider(kind, providers);
    /* Re-adding the managed provider revives it: the removal marker is the
     * only thing keeping the backend sync from materializing it. */
    patch({
      providers: [...providers, provider],
      managedLlamaRemoved: kind === "llamacpp" ? false : draft.managedLlamaRemoved,
    });
    setSelectedId(provider.id);
  };

  const removeProvider = async (provider: ProviderConfig) => {
    const ok = await ask({
      title: t("provider.removeTitle"),
      description: t("provider.removeDescription", { name: provider.name }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    const next = providers.filter((p) => p.id !== provider.id);
    const nextDefault =
      draft.defaultProviderId === provider.id ? (next.find((p) => p.enabled) ?? next[0])?.id ?? "" : draft.defaultProviderId;
    /* Removing the managed provider is an explicit choice: mark it so the
     * backend sync (which materializes the entry whenever local models exist)
     * leaves it hidden until it is added back. */
    patch({
      providers: next,
      defaultProviderId: nextDefault,
      managedLlamaRemoved: draft.managedLlamaRemoved || provider.kind === "llamacpp",
    });
    if (selectedId === provider.id) setSelectedId(null);
  };

  const discoverModels = async (provider: ProviderConfig) => {
    setDiscovering(provider.id);
    try {
      const discovered = await desktop.listProviderModels(provider.id);
      const seen = new Set((provider.models ?? []).map((m) => m.id));
      if (provider.model) seen.add(provider.model);
      const additions = discovered
        .filter((m) => !seen.has(m.id))
        .map((m) => ({ id: m.id, name: m.name || "" }));
      if (additions.length === 0) {
        notify(t("provider.discoveryUpToDate"), "Sparkles");
        return;
      }
      patchProvider(provider.id, { models: [...(provider.models ?? []), ...additions] });
      notify(t("provider.modelsDiscovered", { count: additions.length }), "Sparkles");
    } catch {
      notify(t("provider.discoveryFailed"), "AlertTriangle");
    } finally {
      setDiscovering(null);
    }
  };

  /* The kind switcher offers remote kinds only: switching an existing provider
   * into the managed llama.cpp slot would collide with the single managed entry
   * the backend enforces. Managed llama.cpp is added through the add menu. */
  const remoteKindOptions: { value: string; label: string }[] = [
    { value: "openai-compatible", label: t("provider.openaiCompatible") },
    { value: "ollama", label: t("provider.ollama") },
    { value: "anthropic", label: t("provider.anthropic") },
  ];
  const addKindOptions: { value: string; label: string }[] = canAddManaged
    ? [...remoteKindOptions, { value: "llamacpp", label: t("provider.managedLlamaCpp") }]
    : remoteKindOptions;

  return (
    <div className="mx-auto max-w-[760px]">
      <SectionCard
        title={t("settings.providerHelp")}
        action={
          <Dropdown
            value={""}
            onChange={(v) => addProvider(v as ProviderConfig["kind"])}
            placeholder={t("provider.addProvider")}
            options={addKindOptions}
          />
        }
      >
        <div className="space-y-2">
          {providers.map((provider) => {
            const isDefault = provider.id === draft.defaultProviderId;
            const expanded = selected?.id === provider.id;
            const managed = provider.kind === "llamacpp";
            return (
              <div
                key={provider.id}
                className={cn(
                  "rounded-lg border transition",
                  expanded ? "border-ink-500 bg-ink-850/80" : "border-ink-700/70 bg-ink-850/40",
                )}
              >
                <div className="flex items-center gap-2 px-3 py-2">
                  <button
                    type="button"
                    onClick={() => setSelectedId(expanded ? null : provider.id)}
                    className="flex min-w-0 flex-1 items-center gap-2 text-left"
                  >
                    <Icon
                      name={expanded ? "ChevronDown" : "ChevronRight"}
                      className="h-3.5 w-3.5 shrink-0 text-fg-faint"
                    />
                    <span className={cn("truncate text-[12.5px]", provider.enabled ? "text-fg" : "text-fg-faint line-through")}>
                      {provider.name || provider.id}
                    </span>
                    <span className="shrink-0 rounded border border-ink-700 bg-ink-900 px-1.5 py-px font-mono text-[10px] text-fg-faint">
                      {managed ? t("provider.managedLlamaCpp") : provider.kind}
                    </span>
                    {provider.model && (
                      <span className="hidden truncate font-mono text-[10.5px] text-fg-faint sm:inline">{provider.model}</span>
                    )}
                  </button>
                  {isDefault && (
                    <span className="flex shrink-0 items-center gap-1 rounded-md border border-ink-500 bg-ink-750 px-1.5 py-px text-[10px] font-medium text-fg">
                      <Icon name="Star" className="h-3 w-3" />
                      {t("provider.default")}
                    </span>
                  )}
                  {!isDefault && (
                    <button
                      type="button"
                      onClick={() => patch({ defaultProviderId: provider.id })}
                      className="shrink-0 rounded-md px-1.5 py-1 text-[10.5px] text-fg-faint transition hover:bg-ink-750 hover:text-fg"
                    >
                      {t("provider.makeDefault")}
                    </button>
                  )}
                  <Toggle on={provider.enabled} onChange={(v) => patchProvider(provider.id, { enabled: v })} />
                  <button
                    type="button"
                    onClick={() => void removeProvider(provider)}
                    aria-label={t("common.delete")}
                    className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-fg-faint transition hover:bg-danger/15 hover:text-danger-fg"
                  >
                    <Icon name="Trash2" className="h-3.5 w-3.5" />
                  </button>
                </div>

                {expanded && (
                  <div className="space-y-3 border-t border-seam px-3 py-3">
                    <div className="grid grid-cols-2 gap-3">
                      <Field label={t("provider.name")}>
                        <TextInput value={provider.name} onChange={(v) => patchProvider(provider.id, { name: v })} />
                      </Field>
                      {managed ? (
                        <Field label={t("provider.kind")}>
                          <TextInput value={t("provider.managedLlamaCpp")} onChange={() => undefined} disabled />
                        </Field>
                      ) : (
                        <Field label={t("provider.kind")}>
                          <Dropdown
                            value={provider.kind}
                            onChange={(v) =>
                              patchProvider(provider.id, {
                                kind: v as ProviderConfig["kind"],
                                baseUrl:
                                  v === "ollama" && !provider.baseUrl
                                    ? "http://127.0.0.1:11434"
                                    : provider.baseUrl,
                              })
                            }
                            options={remoteKindOptions}
                          />
                        </Field>
                      )}
                      {provider.kind !== "llamacpp" && (
                        <Field label={t("provider.baseUrl")}>
                          <TextInput
                            value={provider.baseUrl}
                            onChange={(v) => patchProvider(provider.id, { baseUrl: v })}
                            mono
                            placeholder={
                              provider.kind === "ollama"
                                ? "http://127.0.0.1:11434"
                                : provider.kind === "anthropic"
                                  ? "https://api.anthropic.com"
                                  : "https://api.example.com/v1"
                            }
                          />
                        </Field>
                      )}
                      {provider.kind !== "llamacpp" && provider.kind !== "ollama" && (
                        <Field label={t("provider.apiKeyRef")} hint={t("settings.apiKeyHelp")}>
                          <Dropdown
                            value={provider.apiKeyRef ?? ""}
                            onChange={(v) => patchProvider(provider.id, { apiKeyRef: v || undefined })}
                            placeholder={t("settings.noApiKey")}
                            options={[
                              { value: "", label: t("settings.noApiKey") },
                              ...secrets.map((name) => ({ value: name, label: name, icon: "KeyRound" })),
                            ]}
                          />
                        </Field>
                      )}
                    </div>
                    {managed && (
                      <p className="rounded-md border border-ink-700 bg-ink-900/60 px-3 py-2 text-[11.5px] leading-relaxed text-fg-subtle">
                        {t("provider.llamacppNote")}
                      </p>
                    )}

                    <ModelListEditor
                      provider={provider}
                      patchProvider={patchProvider}
                      discovering={discovering === provider.id}
                      onDiscover={() => void discoverModels(provider)}
                      managed={managed}
                    />

                    <CollapsibleBlock
                      title={t("provider.parameters")}
                      badge={paramCount(provider.parameters) > 0 ? String(paramCount(provider.parameters)) : undefined}
                      hint={t("provider.parametersHelp")}
                    >
                      <GenerationParamsEditor
                        value={provider.parameters}
                        onChange={(next) => patchProvider(provider.id, { parameters: next })}
                      />
                    </CollapsibleBlock>
                  </div>
                )}
              </div>
            );
          })}
          {providers.length === 0 && <p className="text-[12px] text-fg-faint">{t("provider.empty")}</p>}
        </div>
      </SectionCard>
    </div>
  );
}

/** Configured model list of one provider: manual entries plus discovery.
 * Managed llama.cpp providers are read-only here: their list mirrors the GGUF
 * files of the Models tab and is synced by the backend on every save. The
 * list itself collapses behind a header row so long catalogs stop eating the
 * page, and every row carries its own generation-parameter overrides. */
function ModelListEditor({
  provider,
  patchProvider,
  discovering,
  onDiscover,
  managed,
}: {
  provider: ProviderConfig;
  patchProvider: (id: string, changes: Partial<ProviderConfig>) => void;
  discovering: boolean;
  onDiscover: () => void;
  managed?: boolean;
}) {
  const { t } = useTranslation();
  const [key, setKey] = useState("");
  const [title, setTitle] = useState("");

  const models = provider.models ?? [];
  const options = [
    { value: "", label: provider.model ? t("provider.defaultModelNamed", { model: provider.model }) : t("provider.defaultModel") },
    ...models.map((m) => ({ value: m.id, label: m.name || m.id })),
  ];

  const addModel = () => {
    const id = key.trim();
    if (!id || models.some((m) => m.id === id)) return;
    patchProvider(provider.id, { models: [...models, { id, name: title.trim() }] });
    setKey("");
    setTitle("");
  };

  return (
    <div className="space-y-2">
      <Field label={t("provider.defaultModelLabel")}>
        <Dropdown
          value={provider.model}
          onChange={(v) => patchProvider(provider.id, { model: v })}
          options={options}
          searchable
          searchPlaceholder={t("common.searchModels")}
          placeholder={t("provider.defaultModel")}
        />
      </Field>
      <CollapsibleBlock
        title={t("provider.models")}
        badge={models.length > 0 ? String(models.length) : undefined}
        hint={managed ? t("provider.llamacppModelsNote") : t("provider.modelsHelp")}
        defaultOpen={models.length > 0 && models.length <= 8}
        action={
          !managed && (
            <Button icon="Search" disabled={discovering} onClick={onDiscover}>
              {discovering ? t("provider.discovering") : t("provider.discover")}
            </Button>
          )
        }
      >
        <div className="space-y-1.5">
          {models.length === 0 && <p className="px-0.5 text-[11.5px] text-fg-faint">{t("provider.noModels")}</p>}
          {models.map((model) => (
            <ModelListRow
              key={model.id}
              model={model}
              provider={provider}
              patchProvider={patchProvider}
              managed={managed}
              parentParams={provider.parameters}
            />
          ))}
        </div>
        {!managed && (
          <div className="flex items-center gap-2 border-t border-seam pt-2">
            <TextInput size="sm" value={key} onChange={setKey} placeholder={t("provider.modelKey")} mono />
            <TextInput size="sm" value={title} onChange={setTitle} placeholder={t("provider.modelTitle")} />
            <Button icon="Plus" disabled={!key.trim()} onClick={addModel}>
              {t("common.add")}
            </Button>
          </div>
        )}
      </CollapsibleBlock>
    </div>
  );
}

/** One model entry of the configured list. Identity fields follow the
 * provider's editability; the generation-parameter overrides stay editable
 * for managed rows too, because local GGUF files carry no discovered values
 * (max tokens, context) to fall back on. */
function ModelListRow({
  model,
  provider,
  patchProvider,
  managed,
  parentParams,
}: {
  model: ModelConfig;
  provider: ProviderConfig;
  patchProvider: (id: string, changes: Partial<ProviderConfig>) => void;
  managed?: boolean;
  parentParams?: GenerationParameters;
}) {
  const { t } = useTranslation();
  const [paramsOpen, setParamsOpen] = useState(false);
  const models = provider.models ?? [];
  const updateModel = (changes: Partial<ModelConfig>) =>
    patchProvider(provider.id, { models: models.map((m) => (m.id === model.id ? { ...m, ...changes } : m)) });
  const count = paramCount(model.parameters);

  return (
    <div className="rounded-md border border-ink-700/60 bg-ink-900/40">
      <div className="flex items-center gap-2 px-2 py-1">
        <span className="w-[42%] shrink-0 truncate rounded-md border border-ink-700 bg-ink-850 px-2 py-1 font-mono text-[11px] text-fg">
          {model.id}
        </span>
        {managed ? (
          <span className="min-w-0 flex-1 truncate px-1 text-[11.5px] text-fg-faint">{model.name || model.id}</span>
        ) : (
          <>
            <TextInput size="sm" value={model.name ?? ""} onChange={(v) => updateModel({ name: v })} placeholder={t("provider.modelTitle")} />
            {provider.model !== model.id && (
              <button
                type="button"
                onClick={() => patchProvider(provider.id, { model: model.id })}
                className="shrink-0 rounded-md px-1.5 py-1 text-[10.5px] text-fg-faint transition hover:bg-ink-750 hover:text-fg"
              >
                {t("provider.makeDefault")}
              </button>
            )}
            <button
              type="button"
              onClick={() => patchProvider(provider.id, { models: models.filter((m) => m.id !== model.id) })}
              aria-label={t("common.delete")}
              className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-fg-faint transition hover:bg-danger/15 hover:text-danger-fg"
            >
              <Icon name="Trash2" className="h-3.5 w-3.5" />
            </button>
          </>
        )}
        <button
          type="button"
          onClick={() => setParamsOpen((o) => !o)}
          aria-label={t("provider.modelParameters")}
          title={t("provider.modelParameters")}
          aria-expanded={paramsOpen}
          className={cn(
            "grid h-7 w-7 shrink-0 place-items-center rounded-md transition hover:bg-ink-750 hover:text-fg",
            count > 0 ? "text-fg" : "text-fg-faint",
          )}
        >
          <span className="relative inline-flex">
            <Icon name="SlidersHorizontal" className="h-3.5 w-3.5" />
            {count > 0 && <span className="absolute -right-1 -top-1 h-1.5 w-1.5 rounded-full bg-success" />}
          </span>
        </button>
      </div>
      {paramsOpen && (
        <div className="border-t border-seam px-2.5 py-2">
          <p className="mb-1.5 text-[10.5px] text-fg-faint">{t("provider.modelParametersHelp")}</p>
          <GenerationParamsEditor value={model.parameters} onChange={(next) => updateModel({ parameters: next })} inherited={parentParams} />
        </div>
      )}
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
  refreshSettings,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
  notify: (text: string, icon?: string) => void;
  /** Re-reads persisted settings so backend-written provider state (the
   * managed llama.cpp entry and its model list) flows into the Providers
   * tab without a manual reload. */
  refreshSettings: () => Promise<void>;
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
      /* Installing selects the model server-side: the managed provider and
       * its model list change with it. */
      await refreshSettings();
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
    await refreshSettings();
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
    await refreshSettings();
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
            <div className="shrink-0 border-b border-seam p-4">
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

            <div className="shrink-0 p-4">
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
            </div>

            {detail.readme && (
              <div className="flex min-h-0 flex-1 flex-col p-4 pt-0">
                <div className="flex min-h-0 flex-1 flex-col rounded-xl border border-ink-700/80 bg-ink-850/40 p-3.5">
                  <h3 className="mb-2 shrink-0 text-[11px] font-medium tracking-[0.08em] text-fg-subtle uppercase">README</h3>
                  <div className="min-h-0 flex-1 overflow-y-auto pr-1 [&_img]:h-auto [&_img]:max-w-full [&_img]:rounded-lg">
                    <MarkdownRenderer text={detail.readme} className="max-w-none" />
                  </div>
                </div>
              </div>
            )}

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

/* Reports whether an installed runtime entry actually carries the build a
 * settings draft pins, so picking a version can preserve the acceleration
 * mode instead of silently resetting it to auto. */
function installedModeAvailable(inst: InstalledLlamaRuntime, mode: RuntimeMode): boolean {
  switch (mode) {
    case "cpu":
      return inst.cpuInstalled;
    case "cuda":
      return inst.cudaInstalled;
    case "vulkan":
      return inst.vulkanInstalled;
    case "hip":
      return inst.hipInstalled;
    default:
      return false;
  }
}

/* Exported for the runtime-panel live regression test: mounts the real panel
 * against a stubbed bridge (scripts/runtime-panel-live-entry.tsx). */
export function RuntimePanel({
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
  /* non-null when the live release lookup failed and no cache could serve a
   * list: the page still shows installed runtimes, so this is a hint, not a
   * dead end. */
  const [releaseError, setReleaseError] = useState<string | null>(null);
  /* non-null when a list was served but not from the live GitHub API: the
   * GitHub releases page (API rate-limited or blocked) or the local cache. */
  const [releaseInfo, setReleaseInfo] = useState<LlamaRuntimeReleaseList | null>(null);
  const [releaseVersion, setReleaseVersion] = useState("");
  const [models, setModels] = useState<LocalModel[]>([]);
  const [busy, setBusy] = useState<string | null>(null);
  const [progress, setProgress] = useState<InstallProgress | null>(null);

  /* Release loading stays independent of the core refresh so a slow live
   * lookup (API timeout, web fallback) never delays the rest of the panel. */
  const loadReleases = useCallback(async (force: boolean) => {
    try {
      const info = force ? await desktop.refreshLlamaRuntimeReleases() : await desktop.listLlamaRuntimeReleases();
      setReleases(info.releases);
      setReleaseInfo(info);
      setReleaseError(null);
      setReleaseVersion((v) => v || info.releases[0]?.version || "");
    } catch (error: unknown) {
      setReleaseInfo(null);
      setReleaseError(error instanceof Error ? error.message : String(error));
    }
  }, []);

  const refresh = useCallback(async () => {
    const [st, cat, mdl] = await Promise.all([
      desktop.getLlamaRuntimeStatus().catch(() => null),
      desktop.getLlamaRuntimeCatalogStatus().catch(() => null),
      desktop.listInstalledLlamaModels().catch(() => []),
    ]);
    setStatus(st);
    setCatalog(cat);
    setModels(mdl);
  }, []);

  useEffect(() => {
    void refresh();
    void loadReleases(false);
  }, [refresh, loadReleases]);

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

  const installed = catalog?.installed ?? [];

  /* Version options lead with the installed runtimes and then merge in the
   * live/cached release list: an offline or rate-limited GitHub API must never
   * hide the runtimes that are already on disk, and the installed-runtime
   * picker must never lead with a version that was never installed. Values are
   * bare versions: the acceleration mode is pinned by the install (the backend
   * writes request.Mode into settings) and resolved at launch, so encoding a
   * mode suffix in the values would keep the configured selection from ever
   * matching — the exact bug that showed the newest release as installed. */
  const versionOptions = useMemo<DropdownOption[]>(() => {
    const modesFor = (inst?: InstalledLlamaRuntime) =>
      [
        inst?.cpuInstalled ? "CPU" : null,
        inst?.cudaInstalled ? "CUDA" : null,
        inst?.vulkanInstalled ? "VULKAN" : null,
        inst?.hipInstalled ? "HIP" : null,
      ].filter(Boolean);
    const options: DropdownOption[] = installed.map((inst) => {
      const modes = modesFor(inst);
      return {
        value: inst.version,
        label: modes.length > 0 ? `${inst.version} · ${modes.join("/")}` : inst.version,
        icon: "Check",
      };
    });
    const seen = new Set(options.map((o) => o.value));
    for (const release of releases) {
      if (seen.has(release.version)) continue;
      options.push({ value: release.version, label: release.version });
    }
    return options;
  }, [installed, releases]);

  /* The field shows exactly the configured version: matched by version (the
   * settings mode can never match a mode suffix), kept visible through a
   * synthetic entry while the catalog loads, and a placeholder — never the
   * newest release — when nothing is configured. */
  const configuredVersion = (rt.runtimeVersion ?? "").trim();
  const runtimeOptions = useMemo(() => {
    if (!configuredVersion || versionOptions.some((o) => o.value === configuredVersion)) return versionOptions;
    return [{ value: configuredVersion, label: configuredVersion }, ...versionOptions];
  }, [versionOptions, configuredVersion]);

  /* Drives the banner under the runtime card: an error when no list could be
   * served at all, an amber note when only a cached list exists, and a subtle
   * info line when the live list came from the GitHub releases page because
   * the API was rate-limited or blocked. */
  const releaseNotice = useMemo(() => {
    if (releaseError) {
      return { kind: "error" as const, icon: "WifiOff", tone: "text-warning", text: t("runtime.releasesUnavailable"), detail: releaseError };
    }
    if (releaseInfo?.source === "github-web") {
      return { kind: "info" as const, icon: "Info", tone: "text-info", text: t("runtime.releasesFromWeb"), detail: releaseInfo.notice };
    }
    if (releaseInfo?.source === "cache" && releaseInfo.fetchedAt) {
      const time = new Date(releaseInfo.fetchedAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
      return { kind: "warn" as const, icon: "WifiOff", tone: "text-warning", text: t("runtime.releasesFromCache", { time }), detail: releaseInfo.notice };
    }
    return null;
  }, [releaseError, releaseInfo, t]);

  return (
    <div className="mx-auto max-w-[720px] space-y-3">
      <SectionCard title={t("runtime.managedTitle")}>
        <div className="space-y-3">
          {releaseNotice && (
            <div className="flex items-start gap-2 rounded-md border border-ink-600 bg-ink-900/70 px-3 py-2 text-[11.5px] leading-relaxed text-fg-subtle">
              <Icon name={releaseNotice.icon} className={cn("mt-px h-3.5 w-3.5 shrink-0", releaseNotice.tone)} />
              <span>
                {releaseNotice.text}
                {releaseNotice.detail && <span className="mt-0.5 block font-mono text-[10.5px] text-fg-faint">{releaseNotice.detail}</span>}
              </span>
            </div>
          )}
          <Field label={t("runtime.contentDirectory")}>
            <TextInput mono value={draft.contentDirectory} onChange={(v) => patch({ contentDirectory: v })} />
          </Field>
          {installed.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="text-[11px] font-semibold tracking-wide text-fg-subtle uppercase">{t("runtime.installedRuntimes")}</span>
              {installed.map((item) => {
                const modes = [
                  item.cpuInstalled ? "CPU" : null,
                  item.cudaInstalled ? "CUDA" : null,
                  item.vulkanInstalled ? "VULKAN" : null,
                  item.hipInstalled ? "HIP" : null,
                ].filter(Boolean);
                const selected = item.version === configuredVersion;
                return (
                  <span
                    key={item.version}
                    className={cn(
                      "rounded-md border px-2 py-px font-mono text-[10.5px]",
                      selected ? "border-ink-500 bg-ink-750 text-fg" : "border-ink-700 bg-ink-900 text-fg-subtle",
                    )}
                  >
                    {item.version} · {modes.join("/") || "?"}
                    {selected && <span className="ml-1 text-fg-faint">· {t("runtime.selected")}</span>}
                  </span>
                );
              })}
            </div>
          )}
          <div className="grid grid-cols-2 gap-3">
            <Field label={t("runtime.installedRuntime")} hint={status?.running ? status.endpoint : undefined}>
              <Dropdown
                value={configuredVersion}
                placeholder={t("runtime.selectRuntime")}
                onChange={(v) => {
                  /* Keep the pinned acceleration mode when the picked version
                   * actually has that build installed; any other choice
                   * resolves automatically at launch. */
                  const next = installed.find((i) => i.version === v);
                  const keepMode: RuntimeMode =
                    rt.mode !== "auto" && next && installedModeAvailable(next, rt.mode) ? rt.mode : "auto";
                  patch({
                    llamaRuntime: {
                      ...rt,
                      runtimeVersion: v,
                      mode: keepMode,
                    },
                  });
                }}
                options={runtimeOptions}
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
            <Button
              icon={busy === "releases" ? "Loader2" : "RefreshCw"}
              spin={busy === "releases"}
              variant="solid"
              disabled={Boolean(busy)}
              onClick={() => {
                setBusy("releases");
                void loadReleases(true).finally(() => setBusy(null));
              }}
            >
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
/* twitch / discord / telegram moved to views/IntegrationPanels.tsx    */
/* (rendered by the Integrations rail section, not by Settings)        */
/* ------------------------------------------------------------------ */

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

  const updateRate = (index: number, key: "providerId" | "model" | "inputUsdPerMillion" | "outputUsdPerMillion", value: string | number) => {
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
                      providerId: draft.defaultProviderId || draft.providers[0]?.id || "",
                      model: "",
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
              <div key={index} className="grid grid-cols-[150px_minmax(0,1fr)_92px_92px_28px] items-center gap-2">
                <Dropdown
                  value={rate.providerId}
                  onChange={(v) => updateRate(index, "providerId", v)}
                  options={draft.providers.map((p) => ({ value: p.id, label: p.name || p.id }))}
                  placeholder={t("metrics.rateProvider")}
                />
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


