/* Browser entry for the chat ToolsPicker live test: mounts the REAL picker
   the way the chat composer does (compact controls row above a message box)
   and exposes scenarios via ?case=:

   - default: a handful of published tool functions, two pre-enabled, one
              enabled-but-missing (deleted function) row, and a search box.
   - bottom : the composer bar sits at the viewport bottom so the portal menu
              must flip upward.

   Bundled to IIFE by render-tools-picker-live.mts and driven headlessly by
   verify-tools-picker.mts. */

import React, { useState } from "react";
import { createRoot } from "react-dom/client";
import { I18nextProvider } from "react-i18next";
import i18n from "../src/i18n";
import { ToolsPicker, type ToolsPickerTool } from "../src/components/ToolsPicker";

const caseName = new URLSearchParams(window.location.search).get("case") ?? "default";

const tools: ToolsPickerTool[] = [
  { id: "fn-weather", name: "Weather lookup", description: "Get the current forecast for one city." },
  { id: "fn-calendar", name: "Calendar events", description: "List calendar events in a date range." },
  { id: "fn-sql", name: "SQL query runner", description: "Run a read-only SQL query on a database." },
  { id: "fn-notes", name: "Note writer", description: "Append a note to the local notes store." },
  { id: "fn-translate", name: "Translator", description: "Translate text between languages." },
  { id: "fn-deleted", name: "", description: "" },
].filter((tool) => tool.name !== "");

function Composer() {
  const [enabled, setEnabled] = useState<string[]>(["fn-weather", "fn-gone"]);
  (window as unknown as Record<string, unknown>).__enabled = enabled;
  return (
    <div
      style={
        caseName === "bottom"
          ? { position: "absolute", left: 0, right: 0, bottom: 24, maxWidth: 640, margin: "0 auto" }
          : { maxWidth: 640, margin: 24 }
      }
    >
      <div className="rounded-2xl border border-ink-700 bg-ink-850 p-3">
        <textarea className="w-full resize-none bg-transparent text-[13px] text-fg" rows={2} />
        <div className="mt-2 flex items-center gap-2">
          <ToolsPicker tools={tools} enabled={enabled} onChange={setEnabled} className="max-w-[180px]" />
        </div>
      </div>
    </div>
  );
}

const root = createRoot(document.getElementById("root")!);
root.render(
  <React.StrictMode>
    <I18nextProvider i18n={i18n}>
      <Composer />
    </I18nextProvider>
  </React.StrictMode>,
);
requestAnimationFrame(() => {
  (window as unknown as Record<string, unknown>).__ready = true;
});
