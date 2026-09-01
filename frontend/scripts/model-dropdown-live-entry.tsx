/* Browser entry for the model-dropdown positioning live test: mounts the REAL
   searchable Dropdown the way the pipeline editor's inspector does — a
   w-[300px] right-hand panel with a full-width Model field — and exposes three
   scenarios via ?case=:

   - wide   : the provider's model list is present from the start and contains
              long model keys, so the menu's natural width far exceeds the
              anchor. Anchored near the right edge, a mis-measured clamp lets
              the menu slip past the right screen edge.
   - async  : the model list arrives 500ms AFTER the menu opens (providers
              loading / discovery refresh). The old menu positioned itself once
              for the narrow list and never re-clamped when the list grew.
   - bottom : the anchor sits near the viewport bottom so the menu must flip
              upward; pins the vertical flip while checking the same bounds.

   Bundled to IIFE by render-model-dropdown-live.mts and driven headlessly by
   verify-model-dropdown-position.mts. */

import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { I18nextProvider } from "react-i18next";
import i18n from "../src/i18n";
import { Dropdown, type DropdownOption } from "../src/components/Dropdown";

const caseName = new URLSearchParams(window.location.search).get("case") ?? "wide";

/* A discovered-models-like list: realistic date-stamped and prefixed keys,
   several wide enough that the menu's natural width beats the 272px anchor. */
const providerModels: DropdownOption[] = [
  { value: "", label: "Provider default · claude-sonnet-4-5-20250929" },
  { value: "claude-sonnet-4-5-20250929", label: "Claude Sonnet 4.5" },
  { value: "claude-opus-4-1-20250805", label: "Claude Opus 4.1" },
  { value: "claude-haiku-4-5", label: "Claude Haiku 4.5" },
  { value: "openrouter/mistralai/mistral-large-2411-turbo", label: "openrouter/mistralai/mistral-large-2411-turbo" },
  { value: "openrouter/deepseek/deepseek-chat-v3-0324", label: "openrouter/deepseek/deepseek-chat-v3-0324" },
  { value: "accounts/fireworks/models/llama-v3-70b-instruct", label: "accounts/fireworks/models/llama-v3-70b-instruct" },
  { value: "my-custom-fine-tuned-claude-sonnet-4-5-deployment-20250929-preview", label: "my-custom-fine-tuned-claude-sonnet-4-5-deployment-20250929-preview" },
  ...Array.from({ length: 36 }, (_, i) => ({
    value: `provider-model-${String(i).padStart(3, "0")}`,
    label: `provider model ${i}`,
  })),
];

/* Before the provider list lands, the picker only offers the default entry —
   exactly what the inspector shows while providers/models load. */
const loadingOptions: DropdownOption[] = [
  { value: "", label: "Provider default model" },
];

function ModelPicker({ options }: { options: DropdownOption[] }) {
  const [model, setModel] = useState("");
  return (
    <label className="block">
      <span className="mb-1 block text-[11px] font-medium text-fg-subtle">Model</span>
      <Dropdown
        value={model}
        options={options}
        searchable
        searchPlaceholder="Search models…"
        onChange={setModel}
      />
    </label>
  );
}

function Page() {
  const [asyncOptions, setAsyncOptions] = useState<DropdownOption[]>(loadingOptions);
  useEffect(() => {
    if (caseName !== "async") return;
    const id = window.setTimeout(() => setAsyncOptions(providerModels), 500);
    return () => window.clearTimeout(id);
  }, []);

  const options = caseName === "async" ? asyncOptions : providerModels;
  return (
    <div className="relative h-screen w-screen overflow-hidden bg-ink-950">
      {/* stand-in for the editor's right-side inspector panel */}
      <aside className="absolute right-0 top-0 flex h-full w-[300px] flex-col gap-4 border-l border-seam bg-ink-900 p-4">
        <p className="text-[12px] font-semibold text-fg">Inspector</p>
        <ModelPicker options={options} />
        {caseName === "bottom" && <div className="flex-1" />}
        {caseName === "bottom" && <ModelPicker options={options} />}
      </aside>
      <p className="absolute left-4 top-4 text-[12px] text-fg-faint">
        editor canvas — case={caseName}
      </p>
    </div>
  );
}

const el = document.getElementById("root");
if (!el) throw new Error("#root missing");
createRoot(el).render(
  <I18nextProvider i18n={i18n}>
    <Page />
  </I18nextProvider>,
);
(window as unknown as Record<string, unknown>).__ready = true;
