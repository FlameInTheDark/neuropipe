/* Verification for the managed llama.cpp provider feature.
   Part 1 pins the SettingsView wiring: "Managed llama.cpp" is offered in the
   add-provider menu (and only there — the kind switcher cannot collide with
   the single managed entry the backend enforces), it disappears from the menu
   once present, its model list is read-only (mirrors the Models tab), and the
   Models tab refreshes settings after install/select/delete so the provider
   list never goes stale. saveSettings adopts the backend's synced view.
   Part 2 pins the backend contract: bindManagedLlamaProvider (routing never
   hijacks the default), syncManagedLlamaModels on every read/save, the lazy
   routeManagedLlama router wired through WithLlamaRouter, and local model
   discovery for the managed provider.
   Part 3 checks the i18n key ships in all four locales.
   Run: npx tsx scripts/verify-managed-provider.mts */

import { readFileSync } from "node:fs";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

const settings = readFileSync(new URL("../src/views/SettingsView.tsx", import.meta.url), "utf8");
const workspace = readFileSync(new URL("../src/features/workspace/useWorkspace.ts", import.meta.url), "utf8");
const desktop = readFileSync(new URL("../../internal/app/desktop.go", import.meta.url), "utf8");
const manager = readFileSync(new URL("../../internal/llm/manager.go", import.meta.url), "utf8");
const catalog = readFileSync(new URL("../../internal/runtime/catalog.go", import.meta.url), "utf8");

/* ------------------------------------------------------------------ */
/* Part 1: SettingsView + workspace wiring                             */
/* ------------------------------------------------------------------ */

/* the add-provider menu offers managed llama.cpp */
check(
  "add menu offers the managed llama.cpp kind",
  settings.includes('{ value: "llamacpp", label: t("provider.managedLlamaCpp") }'),
);
check(
  "add menu is driven by addKindOptions",
  /options=\{addKindOptions\}/.test(settings),
);
/* the menu stops offering it once the single managed entry exists */
check(
  "managed option is hidden once a llamacpp provider exists",
  settings.includes('const canAddManaged = !providers.some((p) => p.kind === "llamacpp");'),
);
check(
  "addKindOptions appends managed only while it can be added",
  settings.includes("const addKindOptions: { value: string; label: string }[] = canAddManaged"),
);

/* the per-provider kind switcher offers remote kinds only */
check(
  "kind switcher uses remoteKindOptions (no managed kind switch)",
  /options=\{remoteKindOptions\}/.test(settings),
);
check(
  "defaultProvider keeps creating the managed entry with the reserved id",
  /case "llamacpp":\s*\n\s*return \{ id: "llama-managed", name: "Managed llama\.cpp", kind, baseUrl: "", model: "", models: \[\], enabled: true \};/.test(
    settings,
  ),
);

/* the managed provider's model list is read-only and synced */
check(
  "ModelListEditor receives the managed flag",
  /<ModelListEditor[\s\S]{0,400}managed=\{managed\}/.test(settings),
);
check(
  "managed header swaps modelsHelp for the llamacppModelsNote hint",
  settings.includes('managed ? t("provider.llamacppModelsNote") : t("provider.modelsHelp")'),
);
check(
  "discover button is hidden for managed providers",
  /action=\{\s*\n\s*!managed && \(\s*\n\s*<Button icon="Search"/.test(settings),
);
check(
  "manual model entry row is hidden for managed providers",
  /\{!managed && \(\s*\n\s*<div className="flex items-center gap-2 border-t border-seam pt-2">/.test(settings),
);
check(
  "managed model rows render read-only (no rename input)",
  settings.includes("{managed ? (") && /managed \?\s*\(\s*\n\s*<span className="min-w-0 flex-1 truncate/.test(settings),
);

/* the Models tab keeps the provider list fresh after backend-side writes */
check(
  "ModelsPanel takes a refreshSettings prop",
  /function ModelsPanel\(\{[\s\S]{0,120}refreshSettings,/.test(settings),
);
check(
  "SettingsView passes workspace.refreshSettings into ModelsPanel",
  /<ModelsPanel[\s\S]{0,220}refreshSettings=\{workspace\.refreshSettings\}/.test(settings),
);
const installWindow = settings.slice(settings.indexOf("const installModel = async () => {"), settings.indexOf("const selectInstalled"));
check(
  "installing a model refreshes settings",
  /await refreshSettings\(\);/.test(installWindow),
  "installModel must await refreshSettings()",
);
check(
  "selecting an installed model refreshes settings",
  /const selectInstalled = async \(path: string\) => \{[\s\S]{0,320}await refreshSettings\(\);/.test(settings),
);
check(
  "deleting an installed model refreshes settings",
  /const deleteInstalled = async \(model: LocalModel\) => \{[\s\S]{0,900}await refreshSettings\(\);/.test(settings),
);

/* saving settings adopts the backend's synced view instead of echoing the draft */
check(
  "saveSettings re-reads settings after save (managed model sync lands in the UI)",
  /await desktop\.saveSettings\(next\);[\s\S]{0,600}saved = await desktop\.getSettings\(\);/.test(workspace),
);
check(
  "saveSettings falls back to the submitted copy when the re-read fails",
  workspace.includes("setSettings(saved ?? next)"),
);

/* ------------------------------------------------------------------ */
/* Part 2: backend contract pins                                       */
/* ------------------------------------------------------------------ */

/* request-time routing must never re-route the default provider */
check(
  "activate keeps forcing the default (explicit user actions)",
  /func activateManagedLlamaProvider\(settings \*domain\.Settings, model, endpoint string\) \{\s*\n\s*bindManagedLlamaProvider\(settings, model, endpoint\)\s*\n\s*settings\.DefaultProviderID = managedLlamaProviderID\s*\n\}/.test(
    desktop,
  ),
);
check(
  "bindManagedLlamaProvider never touches the default provider",
  /func bindManagedLlamaProvider\(settings \*domain\.Settings, model, endpoint string\) \{[\s\S]{0,1400}?\n\}/.test(desktop) &&
    !/func bindManagedLlamaProvider[\s\S]{0,1400}?DefaultProviderID/.test(desktop),
);

/* the managed provider's model list mirrors the Models tab */
check(
  "syncManagedLlamaModels rebuilds the managed model list from installed files",
  desktop.includes("func syncManagedLlamaModels(settings *domain.Settings, files []domain.LocalModel) {"),
);
check(
  "GetSettings syncs the managed model list on every read",
  /func \(d \*Desktop\) GetSettings\(\) domain\.Settings \{[\s\S]{0,700}syncManagedLlamaModels\(&settings, d\.installedLlamaFiles\(\)\)/.test(
    desktop,
  ),
);
check(
  "SaveSettings syncs the managed model list before persisting",
  /normalizeConfiguredProviders\(&settings\)[\s\S]{0,400}syncManagedLlamaModels\(&settings, d\.installedLlamaFiles\(\)\)/.test(
    desktop,
  ),
);
check(
  "sync keeps an uninstalled default model selectable",
  /selected := strings\.TrimSpace\(provider\.Model\)/.test(desktop) &&
    /if selected != "" \{\s*\n\s*if _, exists := seen\[selected\]; !exists \{/.test(desktop),
);
check(
  "sync keeps uninstalled models that carry per-model overrides",
  /if entry\.Parameters == nil && id != selected \{\s*\n\s*continue/.test(desktop),
);

/* lazy routing: start or switch the managed runtime per request */
check(
  "the app installs the llama router on the provider manager",
  desktop.includes("llm.WithLlamaRouter(desktop.routeManagedLlama)"),
);
check(
  "routeManagedLlama serializes model switches",
  desktop.includes("d.llamaRouteMu.Lock()"),
);
check(
  "router resolves the requested model against installed GGUF files",
  desktop.includes("resolveManagedLlamaModel(d.installedLlamaFiles(), target)"),
);
check(
  "router reports an actionable error for uninstalled models",
  desktop.includes('model %q is not installed; download it in Settings, Models'),
);
check(
  "router fast-paths an already-served model",
  /status\.Running && status\.Endpoint != "" && strings\.EqualFold\(status\.Model, target\)/.test(desktop),
);
check(
  "router persists the switch via bindManagedLlamaProvider (default untouched)",
  /bindManagedLlamaProvider\(&settings, file\.Name, started\.Endpoint\)/.test(desktop),
);

/* managed model discovery is a local listing, not a network call */
check(
  "ListProviderModels lists installed files for the managed provider",
  /func \(d \*Desktop\) ListProviderModels\(providerID string\) \(\[\]llm\.ModelInfo, error\) \{[\s\S]{0,900}domain\.ProviderLlamaCPP[\s\S]{0,600}llm\.ModelInfo\{ID: file\.Name, Name: file\.Name\}/.test(
    desktop,
  ),
);
check(
  "catalog exposes the lean InstalledFiles listing (no metadata, no network)",
  catalog.includes("func (c *ModelCatalog) InstalledFiles(ctx context.Context) ([]domain.LocalModel, error) {"),
);

/* the manager routes llamacpp requests through the router with the canonical model */
check(
  "manager defines the LlamaRouter hook",
  manager.includes("type LlamaRouter func(ctx context.Context, model string) (endpoint string, canonical string, err error)"),
);
check(
  "manager offers WithLlamaRouter",
  manager.includes("func WithLlamaRouter(router LlamaRouter) ManagerOption {"),
);
check(
  "Chat routes managed llama.cpp requests",
  (manager.match(/m\.routeLlama\(ctx, provider, model\)/g) ?? []).length >= 2,
);
check(
  "Chat and Converse adopt the router's endpoint and canonical model",
  (manager.match(/provider\.BaseURL, model = endpoint, canonical/g) ?? []).length >= 2,
);
check(
  "without a router the empty-endpoint error stays actionable",
  manager.includes('fmt.Errorf("start managed llama.cpp in Settings before running AI nodes")'),
);

/* ------------------------------------------------------------------ */
/* Part 3: i18n key in every locale                                    */
/* ------------------------------------------------------------------ */

for (const locale of ["en", "de", "fr", "ru"]) {
  const dict = readFileSync(new URL(`../src/i18n/${locale}.ts`, import.meta.url), "utf8");
  check(`provider.llamacppModelsNote present in ${locale}`, dict.includes("    llamacppModelsNote: \""));
}

/* ------------------------------------------------------------------ */

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
