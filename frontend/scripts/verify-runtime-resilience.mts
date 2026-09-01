/* Verification for runtime-page resilience and generation parameters.
   Part 1 pins the release-list cache: a successful lookup persists a
   manifest cache (including the CUDA cudart artifact), any failed or empty
   lookup falls back to it, and only a failure with no cache returns an error
   the Runtime page surfaces.
   Part 2 pins the provider materialization: syncManagedLlamaModels creates
   the managed entry when local models exist, respects the explicit-removal
   marker, and preserves per-model generation overrides; the router honors a
   per-model context size when launching llama-server.
   Part 3 pins the generation-parameter wiring: domain merge semantics, the
   Ollama/OpenAI/Anthropic payload mapping, and range validation.
   Part 4 pins the SettingsView wiring: the Runtime page merges installed
   runtimes into the version picker, shows installed chips and an offline
   banner, the provider list starts collapsed, the model list collapses, and
   provider/model parameter editors exist. Part 5 checks i18n in 4 locales.
   The installed-runtime dropdown value semantics are covered end-to-end by
   verify-runtime-dropdown.mts (live mount against a stubbed bridge).
   Run: npx tsx scripts/verify-runtime-resilience.mts */

import { readFileSync } from "node:fs";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

const settings = readFileSync(new URL("../src/views/SettingsView.tsx", import.meta.url), "utf8");
const types = readFileSync(new URL("../src/lib/types.ts", import.meta.url), "utf8");
const bindings = readFileSync(new URL("../bindings/neuropipe/desktop.ts", import.meta.url), "utf8");
const desktop = readFileSync(new URL("../../internal/app/desktop.go", import.meta.url), "utf8");
const manager = readFileSync(new URL("../../internal/llm/manager.go", import.meta.url), "utf8");
const anthropic = readFileSync(new URL("../../internal/llm/anthropic.go", import.meta.url), "utf8");
const catalog = readFileSync(new URL("../../internal/runtime/catalog.go", import.meta.url), "utf8");
const transport = readFileSync(new URL("../../internal/runtime/transport.go", import.meta.url), "utf8");
const proxyWindows = readFileSync(new URL("../../internal/runtime/proxy_windows.go", import.meta.url), "utf8");
const domain = readFileSync(new URL("../../internal/domain/types.go", import.meta.url), "utf8");

/* ------------------------------------------------------------------ */
/* Part 1: release cache                                               */
/* ------------------------------------------------------------------ */

check(
  "release lookups go through a source-aware resolver",
  /func \(c \*LlamaCatalog\) listReleases\(ctx context\.Context, force bool\)/.test(catalog) &&
    catalog.includes("listing, err := c.listReleases(ctx, false)"),
);
check(
  "successful listings are persisted atomically with their source",
  catalog.includes("func (c *LlamaCatalog) storeReleaseCache(manifests []releaseManifest, source string)") &&
    catalog.includes("os.Rename(path, c.releaseCachePath())"),
);
check(
  "failed or empty API lookups fall back to the web feed and then the cache",
  catalog.includes("webManifests, webErr := c.fetchReleasesFromWeb(ctx)") &&
    catalog.includes("if manifests, record, ok := c.loadReleaseCache(); ok {"),
);
check(
  "the web fallback scrapes the feed and asset pages a browser opens",
  catalog.includes('c.webBase + "/releases.atom"') &&
    catalog.includes('c.webBase + "/releases/expanded_assets/"'),
);
check(
  "a fresh cache suppresses repeat web scrapes while the API is down",
  catalog.includes("releaseCacheFreshFor") &&
    /time\.Since\(record\.FetchedAt\) < releaseCacheFreshFor/.test(catalog),
);
check(
  "a failure with no cache still returns an error",
  catalog.includes('message := "no compatible Windows x64 llama.cpp releases are currently available"'),
);
check(
  "the cached manifest keeps the cudart artifact for offline CUDA installs",
  catalog.includes("type releaseManifest struct {") &&
    /Release\s+domain\.LlamaRuntimeRelease\s+`json:"release"`/.test(catalog) &&
    /Cudart\s+domain\.RuntimeArtifact\s+`json:"cudart"`/.test(catalog),
);
check(
  "API failures name their concrete cause instead of guessing",
  catalog.includes("githubRateLimitDetail(response)") &&
    catalog.includes("describeUnmatchedAPIListing(raw)"),
);
check(
  "outbound HTTP honors the Windows system proxy like the browser",
  catalog.includes("http: newOutboundHTTPClient()") &&
    transport.includes("Proxy:                 outboundProxy") &&
    proxyWindows.includes("Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings"),
);
check(
  "CUDA toolkits accept any version with a driver-compatibility order",
  /bin-win-cuda-\(\[0-9\]\+\\\.\[0-9\]\+\)/.test(catalog) &&
    catalog.includes('preferredCudaToolkits = []string{"12.4", "13.3"}') &&
    /bin-win-\(\?:hip-radeon\|rocm-\[0-9\]\[0-9.\]\*\)/.test(catalog),
);
check(
  "release cache lives in the user-owned runtime root",
  catalog.includes('filepath.Join(c.root, "releases-cache.json")'),
);

/* ------------------------------------------------------------------ */
/* Part 2: provider materialization + context overrides                */
/* ------------------------------------------------------------------ */

check(
  "sync materializes the managed provider when local models exist",
  desktop.includes("settings.Providers = append(slices.Clone(settings.Providers), domain.ProviderConfig{") &&
    /if settings\.ManagedLlamaRemoved \|\| len\(files\) == 0/.test(desktop),
);
check(
  "explicit removal is tracked in settings and honored",
  /ManagedLlamaRemoved\s+bool\s+`json:"managedLlamaRemoved,omitempty"`/.test(domain) &&
    desktop.includes("settings.ManagedLlamaRemoved") &&
    /if settings\.ManagedLlamaRemoved \|\| len\(files\) == 0/.test(desktop),
);
check(
  "bind clears the removal marker so re-adding works",
  /settings\.ManagedLlamaRemoved = false/.test(desktop),
);
check(
  "sync preserves per-model generation overrides through rebuilds",
  desktop.includes("overrides[model.ID] = model.Parameters") &&
    /domain\.ModelConfig\{ID: name, Name: name, Parameters: overrides\[name\]\}/.test(desktop),
);
check(
  "bind preserves provider-level parameters",
  /managed\.Parameters = item\.Parameters/.test(desktop),
);
check(
  "the router honors a per-model context size when launching",
  desktop.includes("if contextSize := d.managedModelContextSize(file.Name); contextSize > 0 {") &&
    desktop.includes("func (d *Desktop) managedModelContextSize(model string) int"),
);
check(
  "explicit runtime start applies the same context override",
  /if contextSize := d\.managedModelContextSize\(filepath\.Base\(runtimeSettings\.ModelPath\)\); contextSize > 0/.test(desktop),
);
check(
  "normalizeProviderModels keeps model parameters",
  /domain\.ModelConfig\{ID: id, Name: strings\.TrimSpace\(model\.Name\), Parameters: model\.Parameters\}/.test(desktop),
);

/* ------------------------------------------------------------------ */
/* Part 3: generation parameters on the wire                           */
/* ------------------------------------------------------------------ */

check(
  "domain defines the generation parameter set",
  domain.includes("type GenerationParameters struct {") &&
    domain.includes("Temperature *float64") &&
    domain.includes("TopK *int") &&
    domain.includes("TopP *float64") &&
    domain.includes("MaxTokens *int") &&
    domain.includes("ContextSize *int"),
);
check(
  "providers and models both carry parameters",
  domain.includes("Parameters *GenerationParameters `json:\"parameters,omitempty\"`"),
);
check(
  "EffectiveParameters merges model overrides over provider values",
  domain.includes("func (p ProviderConfig) EffectiveParameters(model string) GenerationParameters {") &&
    domain.includes("if override.Temperature != nil {"),
);
check(
  "Ollama maps parameters to its options object",
  manager.includes("func ollamaOptions(params domain.GenerationParameters) map[string]any {") &&
    manager.includes('options["num_predict"] = *params.MaxTokens') &&
    manager.includes('options["num_ctx"] = *params.ContextSize'),
);
check(
  "OpenAI-compatible requests apply configured parameters",
  manager.includes("func applyOpenAIParameters(payload map[string]any, params domain.GenerationParameters) {") &&
    manager.includes('payload["max_tokens"] = *params.MaxTokens'),
);
check(
  "chat and converse paths apply effective parameters",
  /chatOllama[\s\S]{0,400}ollamaOptions\(provider\.EffectiveParameters\(model\)\)/.test(manager) &&
    /chatOpenAICompatible[\s\S]{0,600}applyOpenAIParameters\(payload, provider\.EffectiveParameters\(model\)\)/.test(manager) &&
    /converseOllama[\s\S]{0,400}ollamaOptions\(provider\.EffectiveParameters\(model\)\)/.test(manager) &&
    /converseOpenAICompatible[\s\S]{0,600}applyOpenAIParameters\(payload, provider\.EffectiveParameters\(model\)\)/.test(manager),
);
check(
  "Anthropic budget and sampling follow configured parameters",
  anthropic.includes("func anthropicCompletionBudget(params domain.GenerationParameters) int {") &&
    anthropic.includes("func applyAnthropicParameters(payload map[string]any, params domain.GenerationParameters) {") &&
    /chatAnthropic[\s\S]{0,500}anthropicCompletionBudget\(provider\.EffectiveParameters\(model\)\)/.test(anthropic) &&
    /converseAnthropic[\s\S]{0,500}anthropicCompletionBudget\(provider\.EffectiveParameters\(model\)\)/.test(anthropic),
);
check(
  "parameter ranges are validated on save",
  desktop.includes("func validateGenerationParameters(params *domain.GenerationParameters, label string) error {") &&
    desktop.includes("temperature must be between 0 and 2") &&
    desktop.includes("context size must be at least 1024"),
);
check(
  "the 0.2 temperature default is kept only when nothing is configured",
  /chatOpenAICompatible[\s\S]{0,500}"temperature": 0\.2,/.test(manager) &&
    manager.includes("if params.Temperature != nil {\n\t\tpayload[\"temperature\"] = *params.Temperature"),
);

/* ------------------------------------------------------------------ */
/* Part 4: SettingsView wiring                                         */
/* ------------------------------------------------------------------ */

check(
  "runtime panel tracks release-list failures instead of failing silently",
  settings.includes("const [releaseError, setReleaseError] = useState<string | null>(null);") &&
    /catch \(error: unknown\) \{/.test(settings),
);
check(
  "the release-list source drives the offline/web/cache banner",
  settings.includes("releaseNotice && (") &&
    settings.includes('t("runtime.releasesUnavailable")') &&
    settings.includes('t("runtime.releasesFromWeb")') &&
    settings.includes('t("runtime.releasesFromCache"'),
);
check(
  "the release list loads independently so a slow lookup never blocks the panel",
  settings.includes("const loadReleases = useCallback(async (force: boolean) => {") &&
    settings.includes("void loadReleases(false);"),
);
check(
  "the refresh-releases button forces a live lookup past the fresh cache",
  settings.includes("await desktop.refreshLlamaRuntimeReleases()"),
);
check(
  "the release-list binding reports source, timestamp, and notice",
  desktop.includes("func (d *Desktop) ListLlamaRuntimeReleases() (domain.LlamaRuntimeReleaseList, error)") &&
    desktop.includes("func (d *Desktop) RefreshLlamaRuntimeReleases()") &&
    /type LlamaRuntimeReleaseList struct \{/.test(domain) &&
    bindings.includes('call<LlamaRuntimeReleaseList>("ListLlamaRuntimeReleases")') &&
    bindings.includes('call<LlamaRuntimeReleaseList>("RefreshLlamaRuntimeReleases")'),
);
check(
  "version options lead with installed runtimes and merge the release list",
  settings.includes("const installed = catalog?.installed ?? [];") &&
    /for \(const release of releases\) \{\s*\n\s*if \(seen\.has\(release\.version\)\) continue;/.test(settings) &&
    settings.includes("const options: DropdownOption[] = installed.map((inst) => {"),
);
check(
  "the installed-runtime field matches the configured version, not a mode-suffixed value",
  settings.includes("const configuredVersion = (rt.runtimeVersion ?? \"\").trim();") &&
    settings.includes("value={configuredVersion}") &&
    !settings.includes("versionOptions.some((o) => o.value === currentVersion)") &&
    !settings.includes("versionOptions[0]?.value"),
);
check(
  "an unconfigured runtime shows a placeholder instead of the newest release",
  settings.includes('placeholder={t("runtime.selectRuntime")}') &&
    !settings.includes("const currentVersion = `${rt.runtimeVersion ?? releaseVersion}:${rt.mode}`;"),
);
check(
  "the configured version stays visible before the catalog loads",
  settings.includes("return [{ value: configuredVersion, label: configuredVersion }, ...versionOptions];"),
);
check(
  "picking a version keeps the pinned mode when that build is installed",
  settings.includes("function installedModeAvailable(inst: InstalledLlamaRuntime, mode: RuntimeMode): boolean {") &&
    /rt\.mode !== "auto" && next && installedModeAvailable\(next, rt\.mode\) \? rt\.mode : "auto"/.test(settings),
);
check(
  "installed runtimes render as mode chips with a selected marker",
  settings.includes('t("runtime.installedRuntimes")') && settings.includes('t("runtime.selected")'),
);
check(
  "provider panel opens collapsed",
  settings.includes("const selected = providers.find((p) => p.id === selectedId) ?? null;"),
);
check(
  "removing the managed provider sets the removal marker",
  settings.includes('managedLlamaRemoved: draft.managedLlamaRemoved || provider.kind === "llamacpp"'),
);
check(
  "re-adding the managed provider clears the marker",
  settings.includes('managedLlamaRemoved: kind === "llamacpp" ? false : draft.managedLlamaRemoved,'),
);
check(
  "settings draft normalizes the removal marker",
  settings.includes("managedLlamaRemoved: input.managedLlamaRemoved ?? false,"),
);
check(
  "the model list collapses behind a header with a count badge",
  settings.includes("function CollapsibleBlock(") &&
    /title=\{t\("provider\.models"\)\}\s*\n\s*badge=\{models\.length > 0 \? String\(models\.length\) : undefined\}/.test(settings),
);
check(
  "long model lists start collapsed",
  settings.includes("defaultOpen={models.length > 0 && models.length <= 8}"),
);
check(
  "every model row exposes per-model parameter overrides",
  settings.includes("function ModelListRow(") && settings.includes('t("provider.modelParameters")'),
);
check(
  "managed model rows keep editable parameter overrides",
  /managed \? \(\s*\n\s*<span className="min-w-0 flex-1 truncate/.test(settings) &&
    /paramsOpen && \(\s*\n\s*<div className="border-t border-seam px-2\.5 py-2">/.test(settings),
);
check(
  "the provider card carries a collapsible parameters section",
  /<CollapsibleBlock\s*\n\s*title=\{t\("provider\.parameters"\)\}/.test(settings) &&
    /patchProvider\(provider\.id, \{ parameters: next \}\)/.test(settings),
);
check(
  "empty parameter fields stay unset (inherit provider defaults)",
  settings.includes("function OptionalNumberInput(") &&
    settings.includes("onChange(v === undefined ? undefined : v)") === false &&
    /if \(trimmed === ""\) \{\s*\n\s*if \(value !== undefined\) onChange\(undefined\);/.test(settings),
);
check(
  "parameter placeholders show inherited provider values",
  settings.includes('t("provider.paramInherited", { value: String(inherited[field.key]) })'),
);
check(
  "frontend types mirror the generation parameters contract",
  types.includes("export interface GenerationParameters {") &&
    types.includes("parameters?: GenerationParameters;") &&
    types.includes("managedLlamaRemoved?: boolean;"),
);
check(
  "icons include the parameter and offline glyphs",
  readFileSync(new URL("../src/components/icons.tsx", import.meta.url), "utf8").includes("SlidersHorizontal,") &&
    readFileSync(new URL("../src/components/icons.tsx", import.meta.url), "utf8").includes("WifiOff,"),
);

/* ------------------------------------------------------------------ */
/* Part 5: i18n                                                        */
/* ------------------------------------------------------------------ */

const i18nKeys = [
  "provider.parameters",
  "provider.parametersHelp",
  "provider.modelParameters",
  "provider.modelParametersHelp",
  "provider.temperature",
  "provider.topK",
  "provider.topP",
  "provider.maxTokens",
  "provider.contextSize",
  "provider.paramUnset",
  "provider.paramInherited",
  "runtime.installedRuntimes",
  "runtime.selected",
  "runtime.selectRuntime",
  "runtime.releasesUnavailable",
  "runtime.releasesFromWeb",
  "runtime.releasesFromCache",
];
for (const locale of ["en", "de", "fr", "ru"]) {
  const file = readFileSync(new URL(`../src/i18n/${locale}.ts`, import.meta.url), "utf8");
  for (const key of i18nKeys) {
    const leaf = key.split(".").pop();
    check(`${key} present in ${locale}`, file.includes(`${leaf}:`), key);
  }
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
