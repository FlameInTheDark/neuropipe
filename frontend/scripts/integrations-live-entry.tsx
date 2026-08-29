/**
 * Integrations live entry — the REAL IntegrationsView (plus the three real
 * integration panels and DialogHosts for confirmations) rendered against the
 * bridge mock with a minimal workspace. The harness page lets agent-browser
 * click the real buttons: switch integrations in the sidebar, open the Add
 * bot dialog, trust triggers, edit the Twitch Client ID, and save.
 *
 * The harness workspace mimics useWorkspace semantics: refreshTriggers /
 * refreshSettings install NEW object references (like setSettings would) so
 * the view's effects and props see fresh data after every refresh cycle.
 */
import { createRoot } from "react-dom/client";
import "../src/i18n";
import type { Settings, TriggerBinding } from "../src/lib/types";
import type { Workspace } from "../src/features/workspace/useWorkspace";
import { IntegrationsView } from "@/views/IntegrationsView";
import { DialogHosts } from "@/components/DialogHosts";
import { desktop } from "@/lib/bridge";

const notifyLog: string[] = [];

const workspace = {
  settings: null as Settings | null,
  triggers: [] as TriggerBinding[],
  refreshTriggers: async () => {
    workspace.triggers = await desktop.listTriggers();
  },
  refreshSettings: async () => {
    workspace.settings = await desktop.getSettings();
  },
  saveSettings: async (next: Settings) => {
    await desktop.saveSettings(next);
    workspace.settings = next;
  },
  notify: (text: string) => {
    notifyLog.push(text);
  },
} as unknown as Workspace;

function Root() {
  return (
    <div className="h-screen w-screen overflow-hidden bg-ink-1000 text-fg">
      <IntegrationsView workspace={workspace} />
      <DialogHosts />
    </div>
  );
}

async function main() {
  /* pre-load the same way the app's useWorkspace does before views render */
  const [settings, triggers] = await Promise.all([desktop.getSettings(), desktop.listTriggers()]);
  (workspace as { settings: Settings | null }).settings = settings;
  (workspace as { triggers: TriggerBinding[] }).triggers = triggers;

  const container = document.getElementById("root");
  if (!container) throw new Error("#root missing");
  createRoot(container).render(<Root />);
}
void main();

declare global {
  interface Window {
    __notifyLog: string[];
  }
}
window.__notifyLog = notifyLog;
