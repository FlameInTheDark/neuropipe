/* Browser entry for the runtime-panel live regression test: mounts the REAL
   RuntimePanel from SettingsView against the scenario-controlled bridge stub
   (runtime-panel-bridge-stub.ts, aliased over @/lib/bridge by esbuild), so the
   exact component users see renders with controlled installed/release data.

   The reported bug: settings pin runtimeVersion b10205 with mode cuda (what
   InstallLlamaRuntime writes after "Install for CUDA"), every dropdown option
   value used to end in ":auto" and the value fell back to the FIRST option —
   the newest release (b10724) — so the "Installed runtime" field showed a
   version that was never installed.

   Scenarios via ?case= — see runtime-panel-bridge-stub.ts.
   Bundled to IIFE by render-runtime-panel-live.mts, driven headlessly by
   verify-runtime-dropdown.mts. */

import React, { useState } from "react";
import { createRoot } from "react-dom/client";
import { I18nextProvider } from "react-i18next";
import i18n from "../src/i18n";
import { RuntimePanel, normalizeSettings } from "../src/views/SettingsView";
import type { Settings, LlamaRuntimeSettings } from "../src/lib/types";

const caseName = new URLSearchParams(window.location.search).get("case") ?? "pinned";

/* Minimal settings draft shaped like a real save: providers, api and metrics
 * satisfy normalizeSettings; the llama runtime block pins the exact state the
 * reported bug needs. */
const input = {
  language: "en",
  hideToTrayOnClose: false,
  defaultProviderId: "ollama-local",
  contentDirectory: "D:\\Neuropipe",
  retentionDays: 30,
  webhookPort: 7878,
  pluginDirectory: "D:\\Neuropipe\\plugins",
  providers: [
    {
      id: "ollama-local",
      name: "Ollama (local)",
      kind: "ollama",
      baseUrl: "http://127.0.0.1:11434",
      model: "",
      models: [],
      enabled: true,
    },
  ],
  managedLlamaRemoved: false,
  maxConcurrentRuns: 2,
  maxConcurrentLLMRuns: 1,
  llamaRuntime: {
    binaryPath: "D:\\Neuropipe\\runtimes\\llama.cpp\\b10205\\cuda\\llama-server.exe",
    modelPath: "",
    runtimeVersion: caseName === "unconfigured" ? "" : "b10205",
    mode: caseName === "unconfigured" ? "auto" : "cuda",
    contextSize: 8192,
    autoStart: false,
  },
  api: {
    enabled: false,
    bindAddress: "127.0.0.1",
    port: 7878,
    authMode: "token",
    adminEnabled: false,
    tokenRef: "",
    allowedOrigins: [],
  },
  metrics: {
    detailRetentionDays: 30,
    rollupRetentionDays: 365,
    sampleIntervalSeconds: 30,
    priceRates: [],
  },
  twitch: { clientId: "", identities: [] },
  discord: { identities: [] },
  telegram: { identities: [] },
} as unknown as Settings;

const draft = normalizeSettings(input);

function App() {
  const [current, setCurrent] = useState<Settings>(draft);
  return (
    <I18nextProvider i18n={i18n}>
      <div style={{ maxWidth: 720, margin: "24px auto", padding: "0 16px" }}>
        <RuntimePanel
          draft={current}
          patch={(p) => {
            const next = { ...current, ...p };
            setCurrent(next);
            (window as unknown as Record<string, unknown>).__patched = next.llamaRuntime as LlamaRuntimeSettings;
          }}
          notify={() => undefined}
          onSaveDraft={async () => undefined}
        />
      </div>
    </I18nextProvider>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
(window as unknown as Record<string, unknown>).__ready = true;
