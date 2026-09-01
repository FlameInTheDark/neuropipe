/* Scenario-controlled stand-in for @/lib/bridge when the real RuntimePanel is
   mounted in a plain browser (runtime-panel-live-entry.tsx). esbuild aliases
   "@/lib/bridge" to this file, so every desktop.* call the panel makes returns
   controlled data and never touches the Wails runtime. The scenario is chosen
   by the page's ?case= query parameter:

   - pinned        : the reported bug — settings pin b10205 + mode cuda while
                     the newest release is b10724. The installed-runtime
                     dropdown must show b10205, never b10724.
   - unconfigured  : no runtimeVersion configured; the field must show a
                     placeholder instead of the newest release.
   - offline       : the release lookup fails; only installed entries remain.
   - pick-uninstalled: choosing b10724 from the menu patches the draft with
                     mode auto (no installed build to pin).
   - pick-installed: choosing b10206 (cuda + cpu installed) while the draft
                     pins cuda keeps the mode.

   Also tracks in-flight calls so the verifier can wait for the panel's data
   to settle: window.__pending === 0 && window.__bridgeCalls >= 4. */

export function wailsUnavailable(): Error {
  return new Error("Wails runtime unavailable");
}

type Scenario = "pinned" | "unconfigured" | "offline" | "pick-uninstalled" | "pick-installed";

const scenario = (new URLSearchParams(window.location.search).get("case") ?? "pinned") as Scenario;

const runtimeRoot = "D:\\Neuropipe\\runtimes\\llama.cpp";

const artifact = (url: string) => ({ url, size: 1024 });

const release = (version: string) => ({
  version,
  publishedAt: "2026-08-30T12:00:00Z",
  cpu: artifact(`https://github.com/ggml-org/llama.cpp/releases/download/${version}/llama-${version.slice(1)}-bin-win-cpu-x64.zip`),
  cuda: artifact(`https://github.com/ggml-org/llama.cpp/releases/download/${version}/llama-${version.slice(1)}-bin-win-cuda-12.4-x64.zip`),
  vulkan: artifact(`https://github.com/ggml-org/llama.cpp/releases/download/${version}/llama-${version.slice(1)}-bin-win-vulkan-x64.zip`),
  hip: artifact(`https://github.com/ggml-org/llama.cpp/releases/download/${version}/llama-${version.slice(1)}-bin-win-hip-radeon-x64.zip`),
});

const installedFor = (scenario: Scenario) => {
  switch (scenario) {
    case "pick-installed":
      return [
        { version: "b10205", cpuInstalled: false, cudaInstalled: true, vulkanInstalled: false, hipInstalled: false },
        { version: "b10206", cpuInstalled: true, cudaInstalled: true, vulkanInstalled: false, hipInstalled: false },
      ];
    default:
      return [{ version: "b10205", cpuInstalled: false, cudaInstalled: true, vulkanInstalled: false, hipInstalled: false }];
  }
};

const releasesFor = (scenario: Scenario) => {
  switch (scenario) {
    case "offline":
      return [];
    case "pick-installed":
      return [release("b10724"), release("b10723"), release("b10206")];
    default:
      return [release("b10724"), release("b10723"), release("b10722")];
  }
};

function scenarioMode(): string {
  return scenario === "unconfigured" ? "auto" : "cuda";
}

function scenarioVersion(): string {
  return scenario === "unconfigured" ? "" : "b10205";
}

/* Every stubbed method counts as an in-flight bridge call so the verifier can
 * wait for exactly the panel's initial data load to finish. */
function track<T>(fn: () => T): Promise<T> {
  const w = window as unknown as Record<string, number>;
  w.__pending = (w.__pending ?? 0) + 1;
  w.__bridgeCalls = (w.__bridgeCalls ?? 0) + 1;
  return Promise.resolve()
    .then(fn)
    .finally(() => {
      w.__pending = (w.__pending ?? 0) - 1;
    });
}

export const desktop = {
  getLlamaRuntimeStatus: () =>
    track(() => ({
      running: false,
      endpoint: "",
      mode: scenarioMode(),
      model: "",
      message: "",
    })),
  getLlamaRuntimeCatalogStatus: () =>
    track(() => ({
      root: runtimeRoot,
      selectedVersion: scenarioVersion(),
      installed: installedFor(scenario),
    })),
  listInstalledLlamaModels: () => track(() => []),
  listLlamaRuntimeReleases: () => {
    if (scenario === "offline") {
      return track(() => {
        throw new Error(
          "no compatible Windows x64 llama.cpp releases are currently available (the GitHub API response may be empty, rate-limited, or blocked by the network)",
        );
      });
    }
    return track(() => ({
      releases: releasesFor(scenario),
      source: "github-web",
      fetchedAt: "2026-09-01T00:00:00Z",
      notice: "listed from the GitHub releases page",
    }));
  },
  refreshLlamaRuntimeReleases: () =>
    track(() => ({
      releases: releasesFor(scenario),
      source: "github-web",
      fetchedAt: "2026-09-01T00:00:00Z",
      notice: "listed from the GitHub releases page",
    })),
  getInstallProgress: () => track(() => null),
};
