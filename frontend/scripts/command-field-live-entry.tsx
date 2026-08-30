/* Live harness for the Discord Command Trigger's Command picker field:
   renders the real CommandField against the integrations bridge mock and
   verifies that picking a command stores the option schema the Go resolver
   and the canvas twin read. */

import { useState } from "react";
import { createRoot } from "react-dom/client";
import "../src/i18n";
import { CommandField } from "../src/components/discord/CommandField";
import { desktop } from "@/lib/bridge";

function Root() {
  const [picked, setPicked] = useState<unknown>(null);
  return (
    <div className="h-screen w-screen overflow-auto bg-ink-1000 p-8 text-fg">
      <div className="mx-auto max-w-[480px] space-y-4">
        <h1 className="text-[16px] font-semibold">Command picker harness</h1>
        <div id="picker">
          <CommandField value={{}} identityId="dc-1" onChange={(next) => setPicked(next)} />
        </div>
        <pre id="picked" className="rounded-lg border border-ink-700 bg-ink-900 p-3 font-mono text-[11px] text-fg-subtle">
          {JSON.stringify(picked, null, 2)}
        </pre>
      </div>
    </div>
  );
}

const container = document.getElementById("root");
if (!container) throw new Error("#root missing");
createRoot(container).render(<Root />);

void (async () => {
  // Let the command list load.
  await new Promise((resolve) => setTimeout(resolve, 600));
  window.__commandsLoaded = (await desktop.listDiscordApplicationCommands("dc-1", "")).length > 0;
})();
