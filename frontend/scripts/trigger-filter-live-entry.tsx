/* Browser entry for the function-trigger-filter live test: mounts the REAL
   LibraryPanel and NodePicker with libraries produced by the REAL buildLibrary
   (pipeline mode vs function mode) against synthetic definitions mirroring the
   catalog, side by side. Marks elements with stable ids so agent-browser can
   assert what rendered in each mode. */

import React from "react";
import { createRoot } from "react-dom/client";
import { I18nextProvider } from "react-i18next";
import i18n from "../src/i18n";
import { buildLibrary } from "../src/lib/adapters";
import { LibraryPanel } from "../src/components/LibraryPanel";
import { NodePicker } from "../src/components/NodePicker";
import type { NodeDefinition } from "../src/lib/types";
import type { LibraryItem } from "../src/types";

function def(partial: Partial<NodeDefinition> & { type: string; category: string; label: string }): NodeDefinition {
  return {
    description: "",
    icon: "Boxes",
    color: "#fff",
    mode: "impure",
    inputs: [],
    outputs: [],
    fields: [],
    capabilities: [],
    defaultConfig: {},
    source: "builtin",
    ...partial,
  } as NodeDefinition;
}

const definitions: NodeDefinition[] = [
  def({ type: "trigger:button", category: "Triggers", label: "Button Trigger", triggerKind: "button" }),
  def({ type: "twitch:event", category: "Twitch", label: "Twitch Event Trigger", triggerKind: "twitch", mode: "event" }),
  def({ type: "discord:event", category: "Discord", label: "Discord Event Trigger", triggerKind: "discord", mode: "event" }),
  def({ type: "telegram:event", category: "Telegram", label: "Telegram Event Trigger", triggerKind: "telegram", mode: "event" }),
  def({ type: "kv:subscribe", category: "KV Store", label: "KV Subscribe Trigger", triggerKind: "kvsubscribe", mode: "event" }),
  def({ type: "action:http-request", category: "Actions", label: "HTTP Request" }),
  def({ type: "llm:agent", category: "AI", label: "Agent" }),
];

const pipelineLibrary = buildLibrary(definitions, []);
const functionLibrary = buildLibrary(definitions, [], { excludeTriggers: true });

const pick = (item: LibraryItem, category: string) => {
  const log = (window as unknown as Record<string, unknown>).__picks as string[];
  log.push(`${category}:${item.type ?? item.name}`);
};

const el = document.getElementById("root");
if (!el) throw new Error("#root missing");
(window as unknown as Record<string, unknown>).__picks = [];

createRoot(el).render(
  <I18nextProvider i18n={i18n}>
    <div className="flex h-screen gap-4 bg-ink-950 p-4 text-ink-100">
      {/* left: library panel in PIPELINE mode */}
      <div data-testid="panel-pipeline" className="h-full w-[280px] shrink-0">
        <LibraryPanel library={pipelineLibrary} onAdd={pick} />
      </div>
      {/* middle: library panel in FUNCTION mode */}
      <div data-testid="panel-function" className="h-full w-[280px] shrink-0">
        <LibraryPanel library={functionLibrary} onAdd={pick} />
      </div>
      {/* right: the add-node picker (canvas context menu) in FUNCTION mode */}
      <div data-testid="picker-function" className="relative h-full flex-1">
        <NodePicker
          at={{ x: 40, y: 40, gx: 100, gy: 100 }}
          library={functionLibrary}
          snap={true}
          onPick={pick}
          onClose={() => {}}
          onFit={() => {}}
          onToggleSnap={() => {}}
        />
      </div>
    </div>
  </I18nextProvider>,
);
(window as unknown as Record<string, unknown>).__ready = true;
