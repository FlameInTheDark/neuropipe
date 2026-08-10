import { useEffect, useMemo, useRef, useState } from "react";
import {
  BadgeCheck,
  BarChart3,
  Check,
  ChevronRight,
  CircleHelp,
  Code2,
  Copy,
  Cpu,
  Download,
  ExternalLink,
  FolderOpen,
  HardDrive,
  KeyRound,
  Loader2,
  Network,
  Package,
  Play,
  PlugZap,
  RefreshCw,
  Save,
  Search,
  Server,
  Settings2,
  ShieldAlert,
  Square,
  Trash2,
  Workflow,
} from "lucide-react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { PageHeader } from "@/components/PageHeader";
import { MarkdownContent } from "@/components/MarkdownContent";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Progress } from "@/components/ui/progress";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { desktop } from "@/lib/bridge";
import { usePersistedChoice } from "@/lib/preferences";
import type {
  APIStatus,
  InstallProgress,
  LlamaRuntimeCatalogStatus,
  LlamaRuntimeInstallRequest,
  LlamaRuntimeRelease,
  LlamaRuntimeStatus,
  LocalModel,
  ModelDetail,
  ModelFile,
  ModelSearchRequest,
  ModelSearchResult,
  PluginStatus,
  ProviderConfig,
  SecretMetadata,
  Settings,
} from "@/lib/types";
import { cn } from "@/lib/utils";
import { useConfirmationStore } from "@/stores/confirmation";
import { useUIStore } from "@/stores/ui";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import i18n from "@/i18n";
import { languages, type AppLanguage } from "@/i18n/resources";
import { useTranslation } from "react-i18next";

interface SettingsViewProps {
  settings: Settings;
  onSettingsChange: (settings: Settings) => void;
  onRefresh: () => Promise<void>;
}

type SettingsCategory =
  | "general"
  | "provider"
  | "models"
  | "runtime"
  | "api"
  | "execution"
  | "metrics"
  | "extensions"
  | "secrets";
type ModelMode = "catalog" | "installed";
type ModelSort = ModelSearchRequest["sort"];

const categories: ReadonlyArray<{
  id: SettingsCategory;
  labelKey: string;
  icon: typeof Cpu;
  helpKey: string;
}> = [
  {
    id: "general",
    labelKey: "settings.general",
    icon: Settings2,
    helpKey: "settings.generalHelp",
  },
  {
    id: "provider",
    labelKey: "settings.provider",
    icon: Cpu,
    helpKey: "settings.providerHelp",
  },
  {
    id: "models",
    labelKey: "settings.models",
    icon: HardDrive,
    helpKey: "settings.modelsHelp",
  },
  {
    id: "runtime",
    labelKey: "settings.runtime",
    icon: Server,
    helpKey: "settings.runtimeHelp",
  },
  {
    id: "api",
    labelKey: "settings.api",
    icon: Network,
    helpKey: "settings.apiHelp",
  },
  {
    id: "execution",
    labelKey: "settings.execution",
    icon: Workflow,
    helpKey: "settings.executionHelp",
  },
  {
    id: "metrics",
    labelKey: "settings.metrics",
    icon: BarChart3,
    helpKey: "settings.metricsHelp",
  },
  {
    id: "extensions",
    labelKey: "settings.extensions",
    icon: PlugZap,
    helpKey: "settings.extensionsHelp",
  },
  {
    id: "secrets",
    labelKey: "settings.secrets",
    icon: KeyRound,
    helpKey: "settings.secretsHelp",
  },
];

function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

function providerForKind(
  kind: ProviderConfig["kind"],
  current?: ProviderConfig,
): ProviderConfig {
  const previous = current?.kind === kind ? current : undefined;
  if (kind === "llamacpp")
    return {
      id: "llama-managed",
      name: "Managed llama.cpp",
      kind,
      baseUrl: previous?.baseUrl ?? "",
      model: previous?.model ?? "",
      enabled: true,
    };
  if (kind === "openai-compatible")
    return {
      id: "openai-compatible",
      name: "OpenAI-compatible",
      kind,
      baseUrl: previous?.baseUrl ?? "https://api.example.com/v1",
      model: previous?.model ?? "",
      apiKeyRef: previous?.apiKeyRef ?? "",
      enabled: true,
    };
  return {
    id: "ollama-local",
    name: "Local Ollama",
    kind: "ollama",
    baseUrl: previous?.baseUrl ?? "http://127.0.0.1:11434",
    model: previous?.model ?? "",
    enabled: true,
  };
}

function normalizeSettings(settings: Settings): Settings {
  const providers = asArray(settings.providers);
  const selected =
    providers.find((provider) => provider.id === settings.defaultProviderId) ??
    providers[0];
  const provider = providerForKind(selected?.kind ?? "ollama", selected);
  return {
    ...settings,
    language: ["de", "fr", "ru"].includes(settings.language) ? settings.language : "en",
    hideToTrayOnClose: settings.hideToTrayOnClose ?? false,
    contentDirectory: settings.contentDirectory ?? "",
    pluginDirectory: settings.pluginDirectory ?? "",
    defaultProviderId: provider.id,
    providers: [provider],
    api: {
      enabled: settings.api?.enabled ?? false,
      bindAddress: settings.api?.bindAddress ?? "127.0.0.1",
      port: settings.api?.port || settings.webhookPort || 7878,
      authMode: settings.api?.authMode ?? "token",
      allowedOrigins: asArray(settings.api?.allowedOrigins),
      adminEnabled: settings.api?.adminEnabled ?? false,
      exposureAcknowledged: settings.api?.exposureAcknowledged ?? false,
    },
    llamaRuntime: {
      binaryPath: settings.llamaRuntime?.binaryPath ?? "",
      modelPath: settings.llamaRuntime?.modelPath ?? "",
      runtimeVersion: settings.llamaRuntime?.runtimeVersion ?? "",
      mode: settings.llamaRuntime?.mode ?? "auto",
      contextSize: settings.llamaRuntime?.contextSize ?? 8192,
      autoStart: settings.llamaRuntime?.autoStart ?? false,
    },
    metrics: {
      detailRetentionDays: settings.metrics?.detailRetentionDays ?? 30,
      rollupRetentionDays: settings.metrics?.rollupRetentionDays ?? 365,
      sampleIntervalSeconds: settings.metrics?.sampleIntervalSeconds ?? 30,
      priceRates: asArray(settings.metrics?.priceRates),
    },
  };
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(unit === 0 || value >= 10 ? 0 : 1)} ${units[unit]}`;
}

function formatSpeed(bytesPerSecond: number): string {
  return bytesPerSecond > 0
    ? `${formatBytes(bytesPerSecond)}/s`
    : "Calculating speed…";
}
function formatDate(value?: string): string {
  return value
    ? new Date(value).toLocaleDateString(undefined, {
        month: "short",
        day: "numeric",
        year: "numeric",
      })
    : "Unknown";
}
function compactNumber(value: number): string {
  return new Intl.NumberFormat(undefined, {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}
function errorMessage(value: unknown, fallback: string): string {
  return value instanceof Error && value.message
    ? value.message
    : typeof value === "string" && value
      ? value
      : fallback;
}
function apiOriginList(value: string): string[] {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);
}
function isLoopback(address: string): boolean {
  return address === "127.0.0.1" || address === "::1";
}

function normalizeInstallProgress(value: unknown): InstallProgress | null {
  if (!value || typeof value !== "object") return null;
  const progress = value as Partial<InstallProgress>;
  return (progress.kind === "runtime" || progress.kind === "model") &&
    typeof progress.stage === "string"
    ? (progress as InstallProgress)
    : null;
}

function Help({ children }: { children: string }) {
  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex size-4 items-center justify-center rounded-full text-zinc-600 transition hover:bg-zinc-800 hover:text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-400"
          aria-label="More information"
        >
          <CircleHelp className="size-3.5" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-64 p-3 text-xs leading-5 text-zinc-300">
        {children}
      </PopoverContent>
    </Popover>
  );
}

function SectionCard({
  title,
  help,
  children,
  className,
}: {
  title: string;
  help: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("surface rounded-xl p-5", className)}>
      <div className="flex items-center gap-2">
        <h2 className="text-sm font-semibold text-zinc-100">{title}</h2>
        <Help>{help}</Help>
      </div>
      {children}
    </section>
  );
}

function InstallProgressBar({ progress }: { progress: InstallProgress }) {
  const complete =
    progress.stage === "complete" || progress.stage === "installed";
  const failed = progress.stage === "failed";
  const remaining = Math.max(0, progress.totalBytes - progress.downloadedBytes);
  const eta =
    progress.stage === "downloading" &&
    progress.bytesPerSecond > 0 &&
    remaining > 0
      ? `${Math.ceil(remaining / progress.bytesPerSecond)}s remaining`
      : "";
  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-950/80 p-3">
      <div className="flex items-center justify-between gap-3 text-xs">
        <span
          className={cn(
            "truncate font-medium text-zinc-300",
            complete && "text-emerald-300",
            failed && "text-red-300",
          )}
        >
          {progress.label}
        </span>
        <span className="font-mono text-zinc-500">
          {Math.round(progress.percentage)}%
        </span>
      </div>
      <Progress
        className="mt-2"
        value={progress.percentage}
        indicatorClassName={
          failed ? "bg-red-400" : complete ? "bg-emerald-400" : undefined
        }
      />
      <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 font-mono text-[11px] text-zinc-600">
        <span>
          {progress.totalBytes
            ? `${formatBytes(progress.downloadedBytes)} / ${formatBytes(progress.totalBytes)}`
            : "Preparing transfer"}
        </span>
        {progress.stage === "downloading" ? (
          <span>{formatSpeed(progress.bytesPerSecond)}</span>
        ) : null}
        {eta ? <span>~{eta}</span> : null}
      </div>
    </div>
  );
}

function TokenDialog({
  token,
  onClose,
}: {
  token: string;
  onClose: () => void;
}) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    await navigator.clipboard?.writeText(token);
    setCopied(true);
  };
  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/70 p-5 backdrop-blur-sm">
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="api-token-title"
        className="w-full max-w-lg rounded-xl border border-zinc-700 bg-zinc-950 shadow-2xl shadow-black/70"
      >
        <div className="border-b border-zinc-800 px-5 py-4">
          <h2 id="api-token-title" className="text-sm font-semibold">
            New API token
          </h2>
          <p className="mt-1 text-xs leading-5 text-zinc-500">
            Copy it now. Neuropipe stores the token with Windows DPAPI and will
            not show it again.
          </p>
        </div>
        <div className="p-5">
          <code className="block break-all rounded-md border border-zinc-800 bg-zinc-900 px-3 py-2.5 text-xs text-zinc-200">
            {token}
          </code>
        </div>
        <div className="flex justify-end gap-2 border-t border-zinc-800 px-5 py-4">
          <Button variant="outline" onClick={() => void copy()}>
            {copied ? (
              <Check className="size-3.5" />
            ) : (
              <Copy className="size-3.5" />
            )}
            {copied ? "Copied" : "Copy token"}
          </Button>
          <Button onClick={onClose}>Done</Button>
        </div>
      </section>
    </div>
  );
}

function Readme({ markdown, baseURL }: { markdown: string; baseURL: string }) {
  return markdown.trim() ? (
    <MarkdownContent markdown={markdown} baseURL={baseURL} />
  ) : (
    <p className="text-sm text-zinc-500">
      No model card was published for this repository.
    </p>
  );
}

function RepositoryAvatar({
  id,
  author,
  avatarUrl,
}: {
  id: string;
  author?: string;
  avatarUrl?: string;
}) {
  const account = author || id.split("/")[0];
  return (
    <span className="relative flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-zinc-700 bg-zinc-900 text-sm font-semibold text-zinc-300">
      {id.slice(-1).toUpperCase()}
      {avatarUrl ? (
        <img
          src={avatarUrl}
          alt={`${account} avatar`}
          referrerPolicy="no-referrer"
          onError={(event) => {
            event.currentTarget.style.display = "none";
          }}
          className="absolute inset-0 size-full bg-zinc-900 object-cover"
        />
      ) : null}
    </span>
  );
}

function ModelRow({
  model,
  avatarUrl,
  selected,
  pending,
  onSelect,
}: {
  model: ModelSearchResult;
  avatarUrl?: string;
  selected: boolean;
  pending: boolean;
  onSelect: () => void;
}) {
  const tags = asArray(model.tags)
    .filter((tag) => !tag.includes(":"))
    .slice(0, 2);
  return (
    <button
      type="button"
      onClick={onSelect}
      disabled={pending}
      className={cn(
        "flex w-full gap-3 border-b border-zinc-900 px-3 py-3 text-left outline-none transition hover:bg-zinc-900/80 focus-visible:bg-zinc-800 disabled:opacity-50",
        selected && "bg-zinc-800 ring-1 ring-inset ring-zinc-600",
      )}
    >
      <RepositoryAvatar
        id={model.id}
        author={model.author}
        avatarUrl={avatarUrl || model.avatarUrl}
      />
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-1.5">
          <span className="truncate text-sm font-medium text-zinc-200">
            {model.id.split("/").at(-1)}
          </span>
          <BadgeCheck className="size-3.5 shrink-0 text-zinc-500" />
        </span>
        <span className="mt-0.5 block truncate text-xs text-zinc-500">
          {model.author || model.id.split("/")[0]} ·{" "}
          {compactNumber(model.downloads)} downloads
        </span>
        {tags.length ? (
          <span className="mt-1.5 flex gap-1 overflow-hidden">
            {tags.map((tag) => (
              <span
                key={tag}
                className="truncate rounded border border-zinc-800 px-1.5 py-0.5 text-[10px] text-zinc-500"
              >
                {tag}
              </span>
            ))}
          </span>
        ) : null}
      </span>
      {model.lastModified ? (
        <span className="shrink-0 pt-0.5 text-[10px] text-zinc-600">
          {formatDate(model.lastModified)}
        </span>
      ) : null}
    </button>
  );
}

function installedModelTitle(model: LocalModel) {
  return model.repository?.split("/").at(-1) || model.name;
}

function installedModelSubtitle(model: LocalModel) {
  const author = model.author || model.repository?.split("/")[0];
  const source = author
    ? `${author} · ${compactNumber(model.downloads)} downloads`
    : formatBytes(model.size);
  return model.quantization ? `${source} · ${model.quantization}` : source;
}

function GeneralPanel({ language, hideToTrayOnClose, onLanguageChange, onHideToTrayOnCloseChange }: { language: AppLanguage; hideToTrayOnClose: boolean; onLanguageChange: (language: AppLanguage) => void; onHideToTrayOnCloseChange: (enabled: boolean) => void }) {
  const { t } = useTranslation();
  return <div className="mx-auto max-w-2xl space-y-5">
    <section className="surface rounded-xl p-5">
      <h2 className="text-sm font-semibold text-zinc-100">{t("settings.languageTitle")}</h2>
      <p className="mt-1.5 max-w-xl text-xs leading-5 text-zinc-500">{t("settings.languageDescription")}</p>
      <label className="mt-5 block max-w-sm text-xs font-medium text-zinc-300">
        {t("common.language")}
        <Select className="mt-2 w-full" value={language} onValueChange={(value) => onLanguageChange(value as AppLanguage)} options={languages.map((item) => ({ value: item.value, label: t(item.labelKey) }))} ariaLabel={t("common.language")} />
      </label>
    </section>
    <section className="surface flex items-center justify-between gap-5 rounded-xl p-5">
      <div className="min-w-0">
        <h2 className="text-sm font-semibold text-zinc-100">{t("settings.hideToTrayOnCloseTitle")}</h2>
        <p className="mt-1.5 max-w-xl text-xs leading-5 text-zinc-500">{t("settings.hideToTrayOnCloseDescription")}</p>
      </div>
      <Switch label={t("settings.hideToTrayOnCloseTitle")} checked={hideToTrayOnClose} onCheckedChange={onHideToTrayOnCloseChange} />
    </section>
  </div>;
}

export function SettingsView({
  settings,
  onSettingsChange,
  onRefresh,
}: SettingsViewProps) {
  const { setError } = useUIStore();
  const { t } = useTranslation();
  const ask = useConfirmationStore((state) => state.ask);
  const [category, setCategory] = usePersistedChoice<SettingsCategory>(
    "neuropipe.settings.category.v1",
    categories.map((item) => item.id),
    "general",
  );
  const [modelMode, setModelMode] = usePersistedChoice<ModelMode>(
    "neuropipe.models.mode.v1",
    ["catalog", "installed"],
    "catalog",
  );
  const [draft, setDraft] = useState(() => normalizeSettings(settings));
  const [secrets, setSecrets] = useState<SecretMetadata[]>([]);
  const [plugins, setPlugins] = useState<PluginStatus[]>([]);
  const [runtime, setRuntime] = useState<LlamaRuntimeStatus | null>(null);
  const [runtimeCatalog, setRuntimeCatalog] =
    useState<LlamaRuntimeCatalogStatus | null>(null);
  const [runtimeReleases, setRuntimeReleases] = useState<LlamaRuntimeRelease[]>(
    [],
  );
  const [installedModels, setInstalledModels] = useState<LocalModel[]>([]);
  const [apiStatus, setAPIStatus] = useState<APIStatus | null>(null);
  const [modelQuery, setModelQuery] = useState("");
  const [modelSort, setModelSort] = useState<ModelSort>("recommended");
  const [modelResults, setModelResults] = useState<ModelSearchResult[]>([]);
  const [modelDetail, setModelDetail] = useState<ModelDetail | null>(null);
  const [selectedModelFile, setSelectedModelFile] = useState("");
  const [selectedRelease, setSelectedRelease] = useState("");
  const [runtimeInstallProgress, setRuntimeInstallProgress] =
    useState<InstallProgress | null>(null);
  const [modelInstallProgress, setModelInstallProgress] =
    useState<InstallProgress | null>(null);
  const [secretName, setSecretName] = useState("");
  const [secretValue, setSecretValue] = useState("");
  const [apiToken, setAPIToken] = useState("");
  const [busy, setBusy] = useState("");
  const searchSequence = useRef(0);
  const detailSequence = useRef(0);

  const activeProvider = draft.providers[0];
  const selectedExternalModel =
    draft.llamaRuntime.modelPath &&
    !installedModels.some(
      (model) => model.path === draft.llamaRuntime.modelPath,
    );
  const runtimeChoices = useMemo(
    () =>
      (runtimeCatalog?.installed ?? []).flatMap((runtime) =>
        (
          [
            ["cpu", runtime.cpuInstalled],
            ["cuda", runtime.cudaInstalled],
            ["vulkan", runtime.vulkanInstalled],
            ["hip", runtime.hipInstalled],
          ] as const
        )
          .filter(([, installed]) => installed)
          .map(([mode]) => ({
            value: `${runtime.version}:${mode}`,
            label: `${runtime.version} · ${mode.toUpperCase()}`,
          })),
      ),
    [runtimeCatalog],
  );
  const selectedRuntime = draft.llamaRuntime.runtimeVersion
    ? `${draft.llamaRuntime.runtimeVersion}:${draft.llamaRuntime.mode}`
    : "";
  const selectedFile = modelDetail?.files.find(
    (file) => file.name === selectedModelFile,
  );
  const modelBusy =
    busy === "model-search" ||
    busy.startsWith("model-detail") ||
    busy === "model-install";

  useEffect(() => setDraft(normalizeSettings(settings)), [settings]);
  useEffect(() => {
    const stopRuntime = EventsOn(
      "runtime.install.progress",
      (value: unknown) => {
        const progress = normalizeInstallProgress(value);
        if (progress) setRuntimeInstallProgress(progress);
      },
    );
    const stopModel = EventsOn("model.install.progress", (value: unknown) => {
      const progress = normalizeInstallProgress(value);
      if (progress) setModelInstallProgress(progress);
    });
    return () => {
      stopRuntime();
      stopModel();
    };
  }, []);
  useEffect(() => {
    const load = async () => {
      try {
        const [
          nextSecrets,
          nextPlugins,
          nextRuntime,
          nextCatalog,
          nextModels,
          nextAPI,
        ] = await Promise.all([
          desktop.listSecrets(),
          desktop.listPlugins(),
          desktop.getLlamaRuntimeStatus(),
          desktop.getLlamaRuntimeCatalogStatus(),
          desktop.listInstalledLlamaModels(),
          desktop.getAPIStatus(),
        ]);
        setSecrets(asArray(nextSecrets));
        setPlugins(asArray(nextPlugins));
        setRuntime(nextRuntime);
        setRuntimeCatalog(nextCatalog);
        setInstalledModels(asArray(nextModels));
        setAPIStatus(nextAPI);
      } catch (reason) {
        setError(errorMessage(reason, "Unable to load Settings"));
      }
    };
    void load();
  }, [setError]);
  useEffect(() => {
    if (category !== "models" || modelMode !== "catalog" || modelResults.length)
      return;
    void searchModels();
    // searchModels is intentionally event-like and guarded with request sequence state.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [category, modelMode]);
  useEffect(() => {
    const kind = busy.startsWith("runtime-")
      ? "runtime"
      : busy === "model-install"
        ? "model"
        : "";
    if (!kind) return;
    const timer = window.setInterval(() => {
      void desktop
        .getInstallProgress(kind as "runtime" | "model")
        .then(normalizeInstallProgress)
        .then((progress) => {
          if (progress)
            kind === "runtime"
              ? setRuntimeInstallProgress(progress)
              : setModelInstallProgress(progress);
        })
        .catch(() => undefined);
    }, 300);
    return () => window.clearInterval(timer);
  }, [busy]);

  const refreshRuntimeData = async () => {
    const [nextRuntime, nextCatalog, nextModels] = await Promise.all([
      desktop.getLlamaRuntimeStatus(),
      desktop.getLlamaRuntimeCatalogStatus(),
      desktop.listInstalledLlamaModels(),
    ]);
    setRuntime(nextRuntime);
    setRuntimeCatalog(nextCatalog);
    setInstalledModels(asArray(nextModels));
  };
  const save = async () => {
    let next = normalizeSettings(draft);
    if (
      next.api.enabled &&
      (!isLoopback(next.api.bindAddress) || next.api.authMode === "none") &&
      !next.api.exposureAcknowledged
    ) {
      const confirmed = await ask({
        title: "Confirm HTTP API exposure",
        description:
          "The API uses plain HTTP. Keep it loopback-only whenever possible, and use a reverse proxy for TLS when exposing it to a network.",
        confirmLabel: "I understand",
      });
      if (!confirmed) return;
      next = { ...next, api: { ...next.api, exposureAcknowledged: true } };
    }
    try {
      setBusy("settings");
      await desktop.saveSettings(next);
      setDraft(next);
      onSettingsChange(next);
      await i18n.changeLanguage(next.language);
      await Promise.all([
        onRefresh(),
        desktop.getAPIStatus().then(setAPIStatus),
      ]);
    } catch (reason) {
      setError(errorMessage(reason, "Unable to save Settings"));
    } finally {
      setBusy("");
    }
  };

  const clearMetrics = async () => {
    const confirmed = await ask({
      title: "Clear local metrics?",
      description: "This removes local metric facts and daily rollups. Pipelines, execution logs, chats, reports, and Settings remain unchanged.",
      confirmLabel: "Clear metrics",
    });
    if (!confirmed) return;
    try {
      setBusy("metrics-clear");
      await desktop.clearMetrics();
    } catch (reason) {
      setError(errorMessage(reason, "Unable to clear metrics"));
    } finally {
      setBusy("");
    }
  };
  const searchModels = async () => {
    const sequence = ++searchSequence.current;
    try {
      setBusy("model-search");
      const results = asArray(
        await desktop.searchLlamaModels({ query: modelQuery, sort: modelSort }),
      );
      if (sequence !== searchSequence.current) return;
      setModelResults(results);
      const first = results[0];
      if (first) await selectModelRepository(first.id, sequence);
      else {
        setModelDetail(null);
        setSelectedModelFile("");
      }
    } catch (reason) {
      if (sequence === searchSequence.current)
        setError(errorMessage(reason, "Unable to search GGUF models"));
    } finally {
      if (sequence === searchSequence.current) setBusy("");
    }
  };
  const selectModelRepository = async (
    repository: string,
    searchToken?: number,
  ) => {
    const sequence = ++detailSequence.current;
    try {
      setBusy(`model-detail-${repository}`);
      const detail = await desktop.getLlamaModelDetail(repository);
      if (
        sequence !== detailSequence.current ||
        (searchToken !== undefined && searchToken !== searchSequence.current)
      )
        return;
      setModelDetail(detail);
      setSelectedModelFile(
        detail.files.find((file) => file.recommended)?.name ??
          detail.files[0]?.name ??
          "",
      );
    } catch (reason) {
      if (sequence === detailSequence.current)
        setError(errorMessage(reason, "Unable to load model details"));
    } finally {
      if (sequence === detailSequence.current) setBusy("");
    }
  };
  const installModel = async () => {
    if (!modelDetail || !selectedModelFile) return;
    try {
      setBusy("model-install");
      setModelInstallProgress({
        kind: "model",
        stage: "preparing",
        label: "Preparing GGUF model",
        downloadedBytes: 0,
        totalBytes: 0,
        bytesPerSecond: 0,
        percentage: 0,
      });
      await desktop.installLlamaModel({
        repository: modelDetail.id,
        file: selectedModelFile,
      });
      const next = normalizeSettings(await desktop.getSettings());
      setDraft(next);
      onSettingsChange(next);
      setInstalledModels(asArray(await desktop.listInstalledLlamaModels()));
    } catch (reason) {
      setError(errorMessage(reason, "Unable to install this GGUF model"));
    } finally {
      setBusy("");
    }
  };
  const selectInstalledModel = async (path: string) => {
    try {
      setBusy(`select-${path}`);
      await desktop.selectInstalledLlamaModel(path);
      const next = normalizeSettings(await desktop.getSettings());
      setDraft(next);
      onSettingsChange(next);
    } catch (reason) {
      setError(errorMessage(reason, "Unable to select installed model"));
    } finally {
      setBusy("");
    }
  };
  const deleteInstalled = async (model: LocalModel) => {
    if (
      !(await ask({
        title: "Delete installed model?",
        description: `Remove “${model.name}” from Neuropipe’s local content folder? This cannot be undone.`,
        confirmLabel: "Delete model",
      }))
    )
      return;
    try {
      setBusy(`delete-${model.path}`);
      await desktop.deleteInstalledLlamaModel(model.path);
      setInstalledModels(asArray(await desktop.listInstalledLlamaModels()));
      const next = normalizeSettings(await desktop.getSettings());
      setDraft(next);
      onSettingsChange(next);
    } catch (reason) {
      setError(errorMessage(reason, "Unable to delete installed model"));
    } finally {
      setBusy("");
    }
  };
  const chooseContentDirectory = async () => {
    try {
      setBusy("content");
      const folder = await desktop.chooseContentDirectory();
      if (folder)
        setDraft((current) => ({ ...current, contentDirectory: folder }));
    } catch (reason) {
      setError(errorMessage(reason, "Unable to choose a content folder"));
    } finally {
      setBusy("");
    }
  };
  const loadReleases = async () => {
    try {
      setBusy("runtime-releases");
      const releases = asArray(await desktop.listLlamaRuntimeReleases());
      setRuntimeReleases(releases);
      setSelectedRelease((current) => current || releases[0]?.version || "");
    } catch (reason) {
      setError(errorMessage(reason, "Unable to load llama.cpp releases"));
    } finally {
      setBusy("");
    }
  };
  const installRuntime = async (mode: LlamaRuntimeInstallRequest["mode"]) => {
    if (!selectedRelease) return;
    try {
      setBusy(`runtime-${mode}`);
      setRuntimeInstallProgress({
        kind: "runtime",
        stage: "preparing",
        label: "Preparing official runtime",
        downloadedBytes: 0,
        totalBytes: 0,
        bytesPerSecond: 0,
        percentage: 0,
      });
      setRuntimeCatalog(
        await desktop.installLlamaRuntime({ version: selectedRelease, mode }),
      );
      const next = normalizeSettings(await desktop.getSettings());
      setDraft(next);
      onSettingsChange(next);
      await refreshRuntimeData();
    } catch (reason) {
      setError(errorMessage(reason, "Unable to install llama.cpp runtime"));
    } finally {
      setBusy("");
    }
  };
  const startRuntime = async () => {
    try {
      setBusy("runtime");
      await desktop.saveSettings(normalizeSettings(draft));
      setRuntime(await desktop.startLlamaRuntime());
      const next = normalizeSettings(await desktop.getSettings());
      setDraft(next);
      onSettingsChange(next);
    } catch (reason) {
      setError(errorMessage(reason, "Unable to start llama.cpp"));
    } finally {
      setBusy("");
    }
  };
  const saveSecret = async () => {
    try {
      setBusy("secret");
      await desktop.saveSecret(secretName, secretValue);
      setSecretName("");
      setSecretValue("");
      setSecrets(asArray(await desktop.listSecrets()));
    } catch (reason) {
      setError(errorMessage(reason, "Unable to save secret"));
    } finally {
      setBusy("");
    }
  };
  const deleteSecret = async (name: string) => {
    try {
      setBusy(name);
      await desktop.deleteSecret(name);
      setSecrets(asArray(await desktop.listSecrets()));
    } catch (reason) {
      setError(errorMessage(reason, "Unable to delete secret"));
    } finally {
      setBusy("");
    }
  };
  const rotateToken = async () => {
    try {
      setBusy("api-token");
      setAPIToken(await desktop.rotateAPIToken());
      setAPIStatus(await desktop.getAPIStatus());
    } catch (reason) {
      setError(errorMessage(reason, "Unable to create an API token"));
    } finally {
      setBusy("");
    }
  };
  const updateProvider = (
    key: "baseUrl" | "model" | "apiKeyRef",
    value: string,
  ) =>
    setDraft((current) => ({
      ...current,
      providers: [{ ...current.providers[0], [key]: value }],
    }));
  const selectProvider = (kind: ProviderConfig["kind"]) =>
    setDraft((current) => {
      const provider = providerForKind(kind, current.providers[0]);
      return {
        ...current,
        defaultProviderId: provider.id,
        providers: [provider],
      };
    });

  return (
    <section className="flex h-full min-h-0 flex-col">
      <PageHeader
        title={t("common.settings")}
        description={t("settings.description")}
        actions={
          <Button onClick={() => void save()} disabled={busy === "settings"}>
            {busy === "settings" ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Save className="size-4" />
            )}
            {t("settings.save")}
          </Button>
        }
      />
      <div className="min-h-0 flex flex-1">
        <aside className="muted-scroll w-60 min-w-60 max-w-60 shrink-0 overflow-y-auto border-r border-zinc-800 bg-zinc-950/40 p-3">
          {categories.map((item) => {
            const Icon = item.icon;
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => setCategory(item.id)}
                className={cn(
                  "mb-1 flex w-full min-w-0 items-center gap-2.5 rounded-md px-3 py-2 text-left text-sm transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500",
                  category === item.id
                    ? "bg-zinc-800 text-zinc-100"
                    : "text-zinc-500 hover:bg-zinc-900 hover:text-zinc-300",
                )}
              >
                <Icon className="size-4 shrink-0" />
                <span className="min-w-0 flex-1 whitespace-nowrap">
                  {t(item.labelKey)}
                </span>
                {category === item.id ? (
                  <ChevronRight className="size-3.5 shrink-0 text-zinc-500" />
                ) : null}
              </button>
            );
          })}
        </aside>
        <main className="muted-scroll min-w-0 flex-1 overflow-y-auto p-6 lg:p-8">
          {category === "general" ? <GeneralPanel language={draft.language} hideToTrayOnClose={draft.hideToTrayOnClose} onLanguageChange={(language) => { setDraft((current) => ({ ...current, language })); void i18n.changeLanguage(language); }} onHideToTrayOnCloseChange={(hideToTrayOnClose) => setDraft((current) => ({ ...current, hideToTrayOnClose }))} /> : null}
          {category === "provider" ? (
            <ProviderPanel
              provider={activeProvider}
              onProviderKind={selectProvider}
              onChange={updateProvider}
            />
          ) : null}
          {category === "models" ? (
            <ModelsPanel
              mode={modelMode}
              onMode={setModelMode}
              query={modelQuery}
              onQuery={setModelQuery}
              sort={modelSort}
              onSort={setModelSort}
              onSearch={() => void searchModels()}
              results={modelResults}
              detail={modelDetail}
              selectedFile={selectedModelFile}
              onSelectFile={setSelectedModelFile}
              onSelectRepository={(id) => void selectModelRepository(id)}
              busy={modelBusy}
              installBusy={busy === "model-install"}
              onInstall={() => void installModel()}
              installProgress={modelInstallProgress}
              contentDirectory={draft.contentDirectory}
              installed={installedModels}
              activePath={draft.llamaRuntime.modelPath}
              onSelectInstalled={(path) => void selectInstalledModel(path)}
              onDeleteInstalled={(model) => void deleteInstalled(model)}
            />
          ) : null}
          {category === "runtime" ? (
            <RuntimePanel
              draft={draft}
              setDraft={setDraft}
              runtime={runtime}
              runtimeChoices={runtimeChoices}
              selectedRuntime={selectedRuntime}
              selectedExternalModel={Boolean(selectedExternalModel)}
              installedModels={installedModels}
              busy={busy}
              runtimeReleases={runtimeReleases}
              selectedRelease={selectedRelease}
              onSelectedRelease={setSelectedRelease}
              onChooseFolder={() => void chooseContentDirectory()}
              onRefresh={() => void refreshRuntimeData()}
              onStartStop={() =>
                void (runtime?.running
                  ? desktop.stopLlamaRuntime().then(setRuntime)
                  : startRuntime())
              }
              onBrowseReleases={() => void loadReleases()}
              onInstall={(mode) => void installRuntime(mode)}
              progress={runtimeInstallProgress}
            />
          ) : null}
          {category === "api" ? (
            <APIPanel
              draft={draft}
              setDraft={setDraft}
              status={apiStatus}
              busy={busy}
              onRotate={() => void rotateToken()}
            />
          ) : null}
          {category === "execution" ? (
            <ExecutionPanel draft={draft} setDraft={setDraft} />
          ) : null}
          {category === "metrics" ? (
            <MetricsPanel draft={draft} setDraft={setDraft} provider={activeProvider} busy={busy === "metrics-clear"} onClear={() => void clearMetrics()} />
          ) : null}
          {category === "extensions" ? (
            <ExtensionsPanel
              draft={draft}
              setDraft={setDraft}
              plugins={plugins}
              busy={busy}
              onReload={async () => {
                try {
                  setBusy("plugins");
                  setPlugins(asArray(await desktop.rediscoverPlugins()));
                } catch (reason) {
                  setError(
                    errorMessage(reason, "Unable to rediscover plugins"),
                  );
                } finally {
                  setBusy("");
                }
              }}
            />
          ) : null}
          {category === "secrets" ? (
            <SecretsPanel
              secrets={secrets}
              name={secretName}
              value={secretValue}
              setName={setSecretName}
              setValue={setSecretValue}
              busy={busy}
              onSave={() => void saveSecret()}
              onDelete={(name) => void deleteSecret(name)}
            />
          ) : null}
        </main>
      </div>
      {apiToken ? (
        <TokenDialog token={apiToken} onClose={() => setAPIToken("")} />
      ) : null}
    </section>
  );
}

function ProviderPanel({
  provider,
  onProviderKind,
  onChange,
}: {
  provider: ProviderConfig;
  onProviderKind: (kind: ProviderConfig["kind"]) => void;
  onChange: (key: "baseUrl" | "model" | "apiKeyRef", value: string) => void;
}) {
  return (
    <div className="mx-auto max-w-3xl space-y-5">
      <SectionCard
        title="Provider"
        help="Neuropipe intentionally keeps one active provider to make model routing predictable."
      >
        <div className="mt-5 grid gap-4 md:grid-cols-2">
          <label className="text-xs text-zinc-500">
            Provider type
            <Select
              className="mt-1.5"
              value={provider.kind}
              onValueChange={(value) =>
                onProviderKind(value as ProviderConfig["kind"])
              }
              options={[
                { value: "ollama", label: "Ollama" },
                { value: "llamacpp", label: "Managed llama.cpp" },
                { value: "openai-compatible", label: "OpenAI-compatible" },
              ]}
              ariaLabel="Provider type"
            />
          </label>
          {provider.kind === "ollama" ? (
            <>
              <Field
                label="Endpoint"
                value={provider.baseUrl}
                onChange={(value) => onChange("baseUrl", value)}
                placeholder="http://127.0.0.1:11434"
              />
              <Field
                label="Model"
                value={provider.model}
                onChange={(value) => onChange("model", value)}
                placeholder="qwen3"
              />
            </>
          ) : null}
          {provider.kind === "openai-compatible" ? (
            <>
              <Field
                label="Base URL"
                value={provider.baseUrl}
                onChange={(value) => onChange("baseUrl", value)}
                placeholder="https://api.example.com/v1"
              />
              <Field
                label="Model"
                value={provider.model}
                onChange={(value) => onChange("model", value)}
                placeholder="Model ID"
              />
              <Field
                label="Saved API key"
                value={provider.apiKeyRef ?? ""}
                onChange={(value) => onChange("apiKeyRef", value)}
                placeholder="Secret name"
              />
            </>
          ) : null}
          {provider.kind === "llamacpp" ? (
            <div className="md:col-span-2 rounded-lg border border-zinc-800 bg-zinc-950/70 p-4 text-sm text-zinc-400">
              The selected GGUF model and loopback endpoint are controlled in
              Models and Runtime.
            </div>
          ) : null}
        </div>
      </SectionCard>
    </div>
  );
}

function ModelsPanel(props: {
  mode: ModelMode;
  onMode: (value: ModelMode) => void;
  query: string;
  onQuery: (value: string) => void;
  sort: ModelSort;
  onSort: (value: ModelSort) => void;
  onSearch: () => void;
  results: ModelSearchResult[];
  detail: ModelDetail | null;
  selectedFile: string;
  onSelectFile: (value: string) => void;
  onSelectRepository: (value: string) => void;
  busy: boolean;
  installBusy: boolean;
  onInstall: () => void;
  installProgress: InstallProgress | null;
  contentDirectory: string;
  installed: LocalModel[];
  activePath: string;
  onSelectInstalled: (path: string) => void;
  onDeleteInstalled: (model: LocalModel) => void;
}) {
  const {
    mode,
    onMode,
    query,
    onQuery,
    sort,
    onSort,
    onSearch,
    results,
    detail,
    selectedFile,
    onSelectFile,
    onSelectRepository,
    busy,
    installBusy,
    onInstall,
    installProgress,
    contentDirectory,
    installed,
    activePath,
    onSelectInstalled,
    onDeleteInstalled,
  } = props;
  const files = detail?.files ?? [];
  return (
    <div className="mx-auto flex h-[calc(100vh-10.5rem)] min-h-[38rem] max-w-7xl overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950">
      <aside className="flex w-[22rem] shrink-0 flex-col border-r border-zinc-800 bg-zinc-950/70">
        <div className="border-b border-zinc-800 p-3">
          <div className="flex rounded-md border border-zinc-800 bg-zinc-900/70 p-0.5">
            <button
              type="button"
              onClick={() => onMode("catalog")}
              className={cn(
                "flex-1 rounded px-2 py-1.5 text-xs",
                mode === "catalog"
                  ? "bg-zinc-700 text-zinc-100"
                  : "text-zinc-500 hover:text-zinc-300",
              )}
            >
              Catalog
            </button>
            <button
              type="button"
              onClick={() => onMode("installed")}
              className={cn(
                "flex-1 rounded px-2 py-1.5 text-xs",
                mode === "installed"
                  ? "bg-zinc-700 text-zinc-100"
                  : "text-zinc-500 hover:text-zinc-300",
              )}
            >
              Installed ({installed.length})
            </button>
          </div>
          {mode === "catalog" ? (
            <>
              <div className="relative mt-3">
                <Search className="pointer-events-none absolute left-3 top-2.5 size-4 text-zinc-600" />
                <Input
                  value={query}
                  onChange={(event) => onQuery(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") onSearch();
                  }}
                  className="pl-9"
                  placeholder="Search Hugging Face GGUF"
                />
              </div>
              <div className="mt-2 flex gap-2">
                <Select
                  className="flex-1"
                  value={sort}
                  onValueChange={(value) => onSort(value as ModelSort)}
                  options={[
                    { value: "recommended", label: "Recommended" },
                    { value: "downloads", label: "Most downloaded" },
                    { value: "recent", label: "Recently updated" },
                  ]}
                  ariaLabel="Model catalog sort"
                />
                <Button
                  size="sm"
                  variant="outline"
                  disabled={busy}
                  onClick={onSearch}
                >
                  {busy ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : (
                    <RefreshCw className="size-3.5" />
                  )}
                </Button>
              </div>
            </>
          ) : null}
        </div>
        <div className="muted-scroll min-h-0 flex-1 overflow-y-auto">
          {mode === "catalog" ? (
            results.length ? (
              results.map((model) => (
                <ModelRow
                  key={model.id}
                  model={model}
                  avatarUrl={
                    detail?.id === model.id ? detail.avatarUrl : undefined
                  }
                  selected={detail?.id === model.id}
                  pending={busy}
                  onSelect={() => onSelectRepository(model.id)}
                />
              ))
            ) : (
              <p className="px-4 py-8 text-center text-sm text-zinc-600">
                Search public, non-gated GGUF repositories.
              </p>
            )
          ) : installed.length ? (
            installed.map((model) => (
              <div
                key={model.path}
                className={cn(
                  "flex items-center gap-3 border-b border-zinc-900 px-3 py-3",
                  activePath === model.path && "bg-zinc-900",
                )}
              >
                <RepositoryAvatar
                  id={model.repository || model.name}
                  author={model.author}
                  avatarUrl={model.avatarUrl}
                />
                <button
                  type="button"
                  onClick={() => onSelectInstalled(model.path)}
                  className="min-w-0 flex-1 text-left"
                >
                  <span className="block truncate text-sm font-medium text-zinc-200">
                    {installedModelTitle(model)}
                  </span>
                  <span className="mt-1 block truncate text-[11px] text-zinc-500">
                    {installedModelSubtitle(model)}
                  </span>
                  <span className="mt-0.5 block font-mono text-[10px] text-zinc-600">
                    {formatBytes(model.size)}
                  </span>
                </button>
                {activePath === model.path ? (
                  <span className="text-[10px] text-emerald-400">Active</span>
                ) : null}
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={`Delete ${model.name}`}
                  onClick={() => onDeleteInstalled(model)}
                >
                  <Trash2 className="size-3.5 text-red-300" />
                </Button>
              </div>
            ))
          ) : (
            <p className="px-4 py-8 text-center text-sm text-zinc-600">
              No GGUF models are installed.
            </p>
          )}
        </div>
      </aside>
      <section className="muted-scroll min-w-0 flex-1 overflow-x-hidden overflow-y-auto p-6">
        {mode === "installed" ? (
          <InstalledDetail installed={installed} activePath={activePath} />
        ) : detail ? (
          <ModelDetailPane
            detail={detail}
            files={files}
            selectedFile={selectedFile}
            onSelectFile={onSelectFile}
            onInstall={onInstall}
            installBusy={installBusy}
            progress={installProgress}
            contentDirectory={contentDirectory}
          />
        ) : (
          <div className="flex h-full items-center justify-center text-sm text-zinc-600">
            Select a GGUF repository to view its available downloads.
          </div>
        )}
      </section>
    </div>
  );
}

function ModelDetailPane({
  detail,
  files,
  selectedFile,
  onSelectFile,
  onInstall,
  installBusy,
  progress,
  contentDirectory,
}: {
  detail: ModelDetail;
  files: ModelFile[];
  selectedFile: string;
  onSelectFile: (value: string) => void;
  onInstall: () => void;
  installBusy: boolean;
  progress: InstallProgress | null;
  contentDirectory: string;
}) {
  const selected = files.find((file) => file.name === selectedFile);
  return (
    <div className="mx-auto min-w-0 max-w-3xl space-y-7">
      <header className="flex min-w-0 items-start gap-4">
        <RepositoryAvatar
          id={detail.id}
          author={detail.author}
          avatarUrl={detail.avatarUrl}
        />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-3">
            <h2 className="truncate text-2xl font-semibold tracking-tight text-zinc-100">
              {detail.id.split("/").at(-1)}
            </h2>
            <Button
              size="sm"
              variant="outline"
              onClick={() =>
                BrowserOpenURL(`https://huggingface.co/${detail.id}`)
              }
            >
              Open on Hugging Face
              <ExternalLink className="size-3.5" />
            </Button>
          </div>
          <p className="mt-1 break-all text-sm text-zinc-500">{detail.id}</p>
          <div className="mt-3 flex flex-wrap gap-2 text-xs text-zinc-500">
            <span className="rounded-md bg-zinc-900 px-2 py-1">
              {compactNumber(detail.downloads)} downloads
            </span>
            <span className="rounded-md bg-zinc-900 px-2 py-1">
              {detail.likes} likes
            </span>
            <span className="rounded-md bg-zinc-900 px-2 py-1">
              Updated {formatDate(detail.lastModified)}
            </span>
          </div>
        </div>
      </header>
      <section>
        <div className="mb-3 flex items-center gap-2">
          <h3 className="text-sm font-semibold">Download options</h3>
          <Help>
            Each option is a full GGUF model file. Neuropipe resumes downloads
            and checks files against the repository checksum when available.
          </Help>
        </div>
        <Select
          value={selectedFile}
          onValueChange={onSelectFile}
          options={files.map((file) => ({
            value: file.name,
            label: `${file.quantization || "GGUF"} · ${formatBytes(file.size)}`,
            description: file.recommended
              ? `${file.name} · Recommended`
              : file.name,
          }))}
          placeholder="Select a GGUF download"
          ariaLabel="GGUF download option"
        />
        {selected ? (
          <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-zinc-500">
            <span className="font-mono text-zinc-400">{selected.name}</span>
            {selected.sha256 ? (
              <span className="text-emerald-400">SHA-256 verified</span>
            ) : (
              <span>Checksum unavailable</span>
            )}
          </div>
        ) : null}
        <div className="mt-3 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-zinc-800 bg-zinc-900/50 p-3">
          <span className="min-w-0 break-all text-xs text-zinc-500">
            Install to{" "}
            <span className="font-mono text-zinc-400">
              {contentDirectory || "Neuropipe content folder"}
            </span>
          </span>
          <Button disabled={!selectedFile || installBusy} onClick={onInstall}>
            {installBusy ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Download className="size-4" />
            )}
            Install {selected?.size ? formatBytes(selected.size) : "model"}
          </Button>
        </div>
        {progress ? (
          <div className="mt-3">
            <InstallProgressBar progress={progress} />
          </div>
        ) : null}
      </section>
      <section className="rounded-xl border border-zinc-800 bg-zinc-900/35 p-4">
        <h3 className="text-sm font-semibold">Details</h3>
        <div className="mt-3 flex flex-wrap gap-2">
          {asArray(detail.tags)
            .slice(0, 12)
            .map((tag) => (
              <span
                key={tag}
                className="break-all rounded-md border border-zinc-700 bg-zinc-950 px-2 py-1 text-[11px] text-zinc-400"
              >
                {tag}
              </span>
            ))}
        </div>
      </section>
      <section className="min-w-0">
        <h3 className="mb-3 text-sm font-semibold">README</h3>
        <div className="min-w-0 max-w-full overflow-hidden rounded-xl border border-zinc-800 bg-zinc-900/35 p-5">
          <Readme
            markdown={detail.readme ?? ""}
            baseURL={`https://huggingface.co/${detail.id}/resolve/main/`}
          />
        </div>
      </section>
    </div>
  );
}

function InstalledDetail({
  installed,
  activePath,
}: {
  installed: LocalModel[];
  activePath: string;
}) {
  const active = installed.find((model) => model.path === activePath);
  return (
    <div className="mx-auto flex h-full max-w-xl flex-col justify-center text-center">
      <HardDrive className="mx-auto size-8 text-zinc-600" />
      <h2 className="mt-4 text-lg font-semibold">Installed GGUF models</h2>
      <p className="mt-2 text-sm leading-6 text-zinc-500">
        Select a local model in the list to make it active for managed
        llama.cpp.
      </p>
      {active ? (
        <div className="mx-auto mt-5 rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-2 text-xs text-emerald-300">
          Active: {installedModelTitle(active)}
        </div>
      ) : null}
    </div>
  );
}

function RuntimePanel({
  draft,
  setDraft,
  runtime,
  runtimeChoices,
  selectedRuntime,
  selectedExternalModel,
  installedModels,
  busy,
  runtimeReleases,
  selectedRelease,
  onSelectedRelease,
  onChooseFolder,
  onRefresh,
  onStartStop,
  onBrowseReleases,
  onInstall,
  progress,
}: {
  draft: Settings;
  setDraft: React.Dispatch<React.SetStateAction<Settings>>;
  runtime: LlamaRuntimeStatus | null;
  runtimeChoices: { value: string; label: string }[];
  selectedRuntime: string;
  selectedExternalModel: boolean;
  installedModels: LocalModel[];
  busy: string;
  runtimeReleases: LlamaRuntimeRelease[];
  selectedRelease: string;
  onSelectedRelease: (value: string) => void;
  onChooseFolder: () => void;
  onRefresh: () => void;
  onStartStop: () => void;
  onBrowseReleases: () => void;
  onInstall: (mode: LlamaRuntimeInstallRequest["mode"]) => void;
  progress: InstallProgress | null;
}) {
  return (
    <div className="mx-auto max-w-3xl space-y-5">
      <SectionCard
        title="Managed llama.cpp"
        help="Neuropipe manages only llama-server processes it launched and binds them to loopback."
      >
        <div className="mt-5 grid gap-4 md:grid-cols-2">
          <div className="md:col-span-2">
            <div className="flex gap-2">
              <Input
                value={draft.contentDirectory}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    contentDirectory: event.target.value,
                  }))
                }
                placeholder="C:\\Neuropipe"
              />
              <Button
                variant="outline"
                onClick={onChooseFolder}
                disabled={busy === "content"}
              >
                <FolderOpen className="size-4" />
                Browse
              </Button>
            </div>
          </div>
          <label className="text-xs text-zinc-500">
            Installed runtime
            <Select
              className="mt-1.5"
              value={selectedRuntime}
              onValueChange={(value) => {
                const [version, mode] = value.split(":");
                setDraft((current) => ({
                  ...current,
                  llamaRuntime: {
                    ...current.llamaRuntime,
                    runtimeVersion: version,
                    mode: mode as Settings["llamaRuntime"]["mode"],
                  },
                }));
              }}
              options={runtimeChoices}
              placeholder="Choose an installed runtime"
              ariaLabel="Installed managed runtime"
            />
          </label>
          <label className="text-xs text-zinc-500">
            Context tokens
            <Input
              className="mt-1.5"
              type="number"
              min={1024}
              step={1024}
              value={draft.llamaRuntime.contextSize}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  llamaRuntime: {
                    ...current.llamaRuntime,
                    contextSize: Number(event.target.value),
                  },
                }))
              }
            />
          </label>
          <label className="text-xs text-zinc-500 md:col-span-2">
            Installed model
            <Select
              className="mt-1.5"
              value={draft.llamaRuntime.modelPath}
              onValueChange={(path) =>
                setDraft((current) => ({
                  ...current,
                  llamaRuntime: { ...current.llamaRuntime, modelPath: path },
                }))
              }
              options={[
                ...(selectedExternalModel
                  ? [
                      {
                        value: draft.llamaRuntime.modelPath,
                        label: "Previously selected model",
                      },
                    ]
                  : []),
                ...installedModels.map((model) => ({
                  value: model.path,
                  label: installedModelTitle(model),
                  description: `${installedModelSubtitle(model)} · ${formatBytes(model.size)}`,
                })),
              ]}
              placeholder="Choose a model from Models"
              ariaLabel="Installed GGUF model"
            />
          </label>
          <div className="flex items-center justify-between rounded-lg border border-zinc-800 px-3 py-2.5 md:col-span-2">
            <span className="text-sm text-zinc-400">Start with Neuropipe</span>
            <Switch
              label="Start managed llama.cpp with Neuropipe"
              checked={draft.llamaRuntime.autoStart}
              onCheckedChange={(autoStart) =>
                setDraft((current) => ({
                  ...current,
                  llamaRuntime: { ...current.llamaRuntime, autoStart },
                }))
              }
            />
          </div>
        </div>
        <div className="mt-5 flex items-center gap-2">
          <Button disabled={busy === "runtime"} onClick={onStartStop}>
            {busy === "runtime" ? (
              <Loader2 className="size-4 animate-spin" />
            ) : runtime?.running ? (
              <Square className="size-4" />
            ) : (
              <Play className="size-4" />
            )}
            {runtime?.running ? "Stop runtime" : "Start runtime"}
          </Button>
          <Button variant="outline" onClick={onRefresh}>
            <RefreshCw className="size-4" />
            Refresh
          </Button>
          {runtime?.endpoint ? (
            <span className="font-mono text-xs text-zinc-500">
              {runtime.endpoint}
            </span>
          ) : null}
        </div>
      </SectionCard>
      <SectionCard
        title="Official runtime installer"
        help="Downloads are resumable, SHA-256 verified, and atomically installed in your content folder."
      >
        <div className="mt-4 flex gap-2">
          <Select
            className="flex-1"
            value={selectedRelease}
            onValueChange={onSelectedRelease}
            options={runtimeReleases.map((release) => ({
              value: release.version,
              label: release.version,
              description: formatDate(release.publishedAt),
            }))}
            placeholder="Browse releases"
            ariaLabel="Official llama.cpp release"
          />
          <Button
            variant="outline"
            onClick={onBrowseReleases}
            disabled={busy === "runtime-releases"}
          >
            {busy === "runtime-releases" ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <RefreshCw className="size-4" />
            )}
            Browse releases
          </Button>
        </div>
        {runtimeReleases.length ? (
          <div className="mt-3 flex flex-wrap gap-2">
            {(["cpu", "cuda", "vulkan", "hip"] as const).map((mode) => {
              const release = runtimeReleases.find(
                (item) => item.version === selectedRelease,
              );
              const available = Boolean(release?.[mode]?.url);
              return (
                <Button
                  key={mode}
                  size="sm"
                  variant="outline"
                  disabled={!available || busy !== ""}
                  onClick={() => onInstall(mode)}
                >
                  <Download className="size-3.5" />
                  Install {mode.toUpperCase()}
                </Button>
              );
            })}
          </div>
        ) : null}
        {progress ? (
          <div className="mt-3">
            <InstallProgressBar progress={progress} />
          </div>
        ) : null}
      </SectionCard>
    </div>
  );
}

function APIPanel({
  draft,
  setDraft,
  status,
  busy,
  onRotate,
}: {
  draft: Settings;
  setDraft: React.Dispatch<React.SetStateAction<Settings>>;
  status: APIStatus | null;
  busy: string;
  onRotate: () => void;
}) {
  const api = draft.api;
  return (
    <div className="mx-auto max-w-3xl space-y-5">
      <SectionCard
        title="HTTP API"
        help="Runs a configurable local Fiber server. It is disabled by default; it does not provide TLS."
      >
        <div className="mt-5 flex items-center justify-between rounded-lg border border-zinc-800 bg-zinc-950/60 px-3 py-2.5">
          <div>
            <p className="text-sm font-medium text-zinc-300">Enable API</p>
            <p className="mt-0.5 text-xs text-zinc-600">
              Webhooks are available only while the API is running.
            </p>
          </div>
          <Switch
            label="Enable HTTP API"
            checked={api.enabled}
            onCheckedChange={(enabled) =>
              setDraft((current) => ({
                ...current,
                api: { ...current.api, enabled },
              }))
            }
          />
        </div>
        <div
          className={cn(
            "mt-4 grid gap-4 md:grid-cols-2",
            !api.enabled && "pointer-events-none opacity-45",
          )}
        >
          <Field
            label="Bind address"
            value={api.bindAddress}
            onChange={(bindAddress) =>
              setDraft((current) => ({
                ...current,
                api: { ...current.api, bindAddress },
              }))
            }
            placeholder="127.0.0.1"
          />
          <label className="text-xs text-zinc-500">
            Port
            <Input
              className="mt-1.5"
              type="number"
              min={1024}
              max={65535}
              value={api.port}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  api: { ...current.api, port: Number(event.target.value) },
                }))
              }
            />
          </label>
          <label className="text-xs text-zinc-500">
            Authentication
            <Select
              className="mt-1.5"
              value={api.authMode}
              onValueChange={(authMode) =>
                setDraft((current) => ({
                  ...current,
                  api: {
                    ...current.api,
                    authMode: authMode as Settings["api"]["authMode"],
                    adminEnabled:
                      authMode === "none" ? false : current.api.adminEnabled,
                  },
                }))
              }
              options={[
                { value: "token", label: "Bearer token" },
                {
                  value: "none",
                  label: "No authentication",
                  description: "Operational endpoints only",
                },
              ]}
              ariaLabel="API authentication"
            />
          </label>
          <div className="flex items-end gap-2">
            <Button
              variant="outline"
              disabled={busy === "api-token"}
              onClick={onRotate}
            >
              {busy === "api-token" ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <KeyRound className="size-4" />
              )}
              {status?.tokenConfigured ? "Rotate token" : "Create token"}
            </Button>
            {status?.tokenConfigured ? (
              <span className="pb-2 text-xs text-emerald-400">
                Token configured
              </span>
            ) : null}
          </div>
          <label className="text-xs text-zinc-500 md:col-span-2">
            Allowed CORS origins
            <Input
              className="mt-1.5"
              value={api.allowedOrigins.join(", ")}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  api: {
                    ...current.api,
                    allowedOrigins: apiOriginList(event.target.value),
                  },
                }))
              }
              placeholder="https://dashboard.example.com"
            />
          </label>
          {api.authMode === "token" ? (
            <div className="flex items-center justify-between rounded-lg border border-zinc-800 px-3 py-2.5 md:col-span-2">
              <span className="text-sm text-zinc-400">Administrative API</span>
              <Switch
                label="Enable administrative API"
                checked={api.adminEnabled}
                onCheckedChange={(adminEnabled) =>
                  setDraft((current) => ({
                    ...current,
                    api: { ...current.api, adminEnabled },
                  }))
                }
              />
            </div>
          ) : (
            <div className="md:col-span-2 rounded-lg border border-amber-500/20 bg-amber-500/5 px-3 py-2.5 text-xs leading-5 text-amber-200">
              Unauthenticated mode disables administrative endpoints. Keep this
              mode loopback-only.
            </div>
          )}
        </div>
        <div className="mt-4 flex items-center gap-2 rounded-lg border border-zinc-800 px-3 py-2.5">
          <span
            className={
              status?.running
                ? "size-2 rounded-full bg-emerald-400"
                : "size-2 rounded-full bg-zinc-600"
            }
          />
          <span className="text-xs text-zinc-400">
            {status?.running
              ? status.endpoint
              : status?.message || "API is disabled"}
          </span>
        </div>
        {api.enabled && !isLoopback(api.bindAddress) ? (
          <div className="mt-3 flex gap-2 rounded-lg border border-amber-500/20 bg-amber-500/5 p-3 text-xs leading-5 text-amber-100">
            <ShieldAlert className="mt-0.5 size-4 shrink-0" />
            Use a reverse proxy to terminate TLS before exposing this HTTP
            listener outside your machine.
          </div>
        ) : null}
      </SectionCard>
    </div>
  );
}

function ExecutionPanel({
  draft,
  setDraft,
}: {
  draft: Settings;
  setDraft: React.Dispatch<React.SetStateAction<Settings>>;
}) {
  return (
    <div className="mx-auto max-w-3xl">
      <SectionCard
        title="Execution"
        help="Limits protect local resources. Per-pipeline overlapping runs are still skipped and recorded."
      >
        <div className="mt-5 grid gap-4 md:grid-cols-3">
          <NumberField
            label="Retention days"
            value={draft.retentionDays}
            onChange={(retentionDays) =>
              setDraft((current) => ({ ...current, retentionDays }))
            }
            min={1}
          />
          <NumberField
            label="Parallel pipeline runs"
            value={draft.maxConcurrentRuns}
            onChange={(maxConcurrentRuns) =>
              setDraft((current) => ({ ...current, maxConcurrentRuns }))
            }
            min={1}
            max={16}
          />
          <NumberField
            label="Parallel LLM runs"
            value={draft.maxConcurrentLLMRuns}
            onChange={(maxConcurrentLLMRuns) =>
              setDraft((current) => ({ ...current, maxConcurrentLLMRuns }))
            }
            min={1}
            max={8}
          />
        </div>
      </SectionCard>
    </div>
  );
}

function MetricsPanel({
  draft,
  setDraft,
  provider,
  busy,
  onClear,
}: {
  draft: Settings;
  setDraft: React.Dispatch<React.SetStateAction<Settings>>;
  provider: ProviderConfig;
  busy: boolean;
  onClear: () => void;
}) {
  const rates = draft.metrics.priceRates;
  const updateRate = (index: number, patch: Partial<(typeof rates)[number]>) => setDraft((current) => ({ ...current, metrics: { ...current.metrics, priceRates: current.metrics.priceRates.map((rate, currentIndex) => currentIndex === index ? { ...rate, ...patch } : rate) } }));
  const addRate = () => setDraft((current) => ({ ...current, metrics: { ...current.metrics, priceRates: [...current.metrics.priceRates, { providerId: provider.id, model: provider.model || "", inputUsdPerMillion: 0, outputUsdPerMillion: 0 }] } }));
  const removeRate = (index: number) => setDraft((current) => ({ ...current, metrics: { ...current.metrics, priceRates: current.metrics.priceRates.filter((_, currentIndex) => currentIndex !== index) } }));
  return (
    <div className="mx-auto max-w-3xl space-y-5">
      <SectionCard title="Local metrics" help="Neuropipe stores numerical timing and outcome facts locally. Prompts, responses, packets, secrets, URLs, API headers, and IP addresses are never collected.">
        <div className="mt-5 grid gap-4 md:grid-cols-3">
          <NumberField label="Detailed facts (days)" value={draft.metrics.detailRetentionDays} min={1} max={365} onChange={(detailRetentionDays) => setDraft((current) => ({ ...current, metrics: { ...current.metrics, detailRetentionDays } }))} />
          <NumberField label="Daily rollups (days)" value={draft.metrics.rollupRetentionDays} min={30} max={3650} onChange={(rollupRetentionDays) => setDraft((current) => ({ ...current, metrics: { ...current.metrics, rollupRetentionDays } }))} />
          <NumberField label="Process sample (seconds)" value={draft.metrics.sampleIntervalSeconds} min={10} max={300} onChange={(sampleIntervalSeconds) => setDraft((current) => ({ ...current, metrics: { ...current.metrics, sampleIntervalSeconds } }))} />
        </div>
        <div className="mt-4 flex items-center justify-between gap-4 rounded-lg border border-zinc-800 bg-zinc-950/70 px-3 py-2.5"><p className="text-xs leading-5 text-zinc-500">Only Neuropipe and its managed llama.cpp process are sampled for CPU and memory.</p><Button size="sm" variant="outline" onClick={onClear} disabled={busy}>{busy ? <Loader2 className="size-3.5 animate-spin" /> : <Trash2 className="size-3.5" />}Clear metrics</Button></div>
      </SectionCard>
      <SectionCard title="Cost estimates" help="Optional rates estimate hosted-model spend from provider-reported token counts. Local Ollama and managed llama.cpp calls are shown as local, not provider billing.">
        <div className="mt-5 space-y-2">
          {rates.map((rate, index) => <div key={`${rate.providerId}-${rate.model}-${index}`} className="grid gap-2 rounded-lg border border-zinc-800 bg-zinc-950/60 p-3 md:grid-cols-[minmax(9rem,1fr)_minmax(8rem,.8fr)_minmax(8rem,.8fr)_auto]">
            <Input value={rate.model} onChange={(event) => updateRate(index, { model: event.target.value })} placeholder="Model name" aria-label="Price rate model" />
            <label className="text-[11px] text-zinc-500">Input / 1M<Input className="mt-1" type="number" min={0} step="0.0001" value={rate.inputUsdPerMillion} onChange={(event) => updateRate(index, { inputUsdPerMillion: Number(event.target.value) })} aria-label="Input dollars per million tokens" /></label>
            <label className="text-[11px] text-zinc-500">Output / 1M<Input className="mt-1" type="number" min={0} step="0.0001" value={rate.outputUsdPerMillion} onChange={(event) => updateRate(index, { outputUsdPerMillion: Number(event.target.value) })} aria-label="Output dollars per million tokens" /></label>
            <Button size="sm" variant="ghost" className="self-end text-zinc-500 hover:text-red-300" onClick={() => removeRate(index)} aria-label={`Remove ${rate.model || "price rate"}`}><Trash2 className="size-3.5" /></Button>
          </div>)}
          {rates.length === 0 ? <p className="rounded-lg border border-dashed border-zinc-800 px-3 py-4 text-xs text-zinc-500">No hosted-model price rates. Usage remains visible, but cost stays unpriced.</p> : null}
        </div>
        <Button className="mt-4" size="sm" variant="outline" onClick={addRate}>Add price rate</Button>
      </SectionCard>
    </div>
  );
}

function ExtensionsPanel({
  draft,
  setDraft,
  plugins,
  busy,
  onReload,
}: {
  draft: Settings;
  setDraft: React.Dispatch<React.SetStateAction<Settings>>;
  plugins: PluginStatus[];
  busy: string;
  onReload: () => Promise<void>;
}) {
  return (
    <div className="mx-auto max-w-3xl">
      <SectionCard
        title="Extensions"
        help="Plugins run as local bundles and declare their own nodes and capabilities."
      >
        <Input
          className="mt-5"
          value={draft.pluginDirectory}
          onChange={(event) =>
            setDraft((current) => ({
              ...current,
              pluginDirectory: event.target.value,
            }))
          }
        />
        <div className="mt-4 divide-y divide-zinc-800 rounded-lg border border-zinc-800">
          {plugins.length ? (
            plugins.map((plugin) => (
              <div
                key={plugin.path}
                className="flex items-center justify-between gap-3 px-3 py-3 text-xs"
              >
                <span className="min-w-0">
                  <strong className="truncate text-zinc-300">
                    {plugin.name}
                  </strong>
                  <span className="ml-2 text-zinc-600">
                    v{plugin.version} · {plugin.nodeCount} nodes
                  </span>
                </span>
                <span
                  className={
                    plugin.healthy ? "text-emerald-400" : "text-red-300"
                  }
                >
                  {plugin.healthy ? "Healthy" : plugin.error || "Unavailable"}
                </span>
              </div>
            ))
          ) : (
            <p className="px-3 py-4 text-sm text-zinc-600">
              No plugin bundles found.
            </p>
          )}
        </div>
        <Button
          className="mt-4"
          variant="outline"
          onClick={() => void onReload()}
          disabled={busy === "plugins"}
        >
          {busy === "plugins" ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <RefreshCw className="size-4" />
          )}
          Rediscover plugins
        </Button>
      </SectionCard>
    </div>
  );
}

function SecretsPanel({
  secrets,
  name,
  value,
  setName,
  setValue,
  busy,
  onSave,
  onDelete,
}: {
  secrets: SecretMetadata[];
  name: string;
  value: string;
  setName: (value: string) => void;
  setValue: (value: string) => void;
  busy: string;
  onSave: () => void;
  onDelete: (name: string) => void;
}) {
  return (
    <div className="mx-auto max-w-3xl">
      <SectionCard
        title="Secrets"
        help="Values are encrypted with Windows DPAPI and never return to the renderer."
      >
        <div className="mt-5 grid gap-2 md:grid-cols-[1fr_1.5fr_auto]">
          <Input
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="Secret name"
          />
          <Input
            type="password"
            value={value}
            onChange={(event) => setValue(event.target.value)}
            placeholder="Secret value"
          />
          <Button
            onClick={onSave}
            disabled={busy === "secret" || !name.trim() || !value.trim()}
          >
            {busy === "secret" ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Check className="size-4" />
            )}
            Save
          </Button>
        </div>
        <div className="mt-4 divide-y divide-zinc-800 rounded-lg border border-zinc-800">
          {secrets.length ? (
            secrets.map((secret) => (
              <div
                key={secret.name}
                className="flex items-center justify-between px-3 py-2.5"
              >
                <span className="font-mono text-sm text-zinc-300">
                  {secret.name}
                </span>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onDelete(secret.name)}
                  disabled={busy === secret.name}
                >
                  <Trash2 className="size-3.5 text-red-300" />
                </Button>
              </div>
            ))
          ) : (
            <p className="px-3 py-4 text-sm text-zinc-600">No secrets saved.</p>
          )}
        </div>
      </SectionCard>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  return (
    <label className="text-xs text-zinc-500">
      {label}
      <Input
        className="mt-1.5"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
      />
    </label>
  );
}
function NumberField({
  label,
  value,
  onChange,
  min,
  max,
}: {
  label: string;
  value: number;
  onChange: (value: number) => void;
  min: number;
  max?: number;
}) {
  return (
    <label className="text-xs text-zinc-500">
      {label}
      <Input
        className="mt-1.5"
        type="number"
        min={min}
        max={max}
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
      />
    </label>
  );
}
