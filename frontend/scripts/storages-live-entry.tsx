/**
 * Storages live entry — the REAL StoragesView (plus StorageConnectionModal
 * and StorageBrowser through it) rendered against the bridge mock. The
 * harness page lets agent-browser click the real buttons: select a
 * connection, navigate folders, open the new-connection dialog, switch
 * engines, and exercise the browser toolbar.
 */
import { createRoot } from "react-dom/client";
import "../src/i18n";
import { StoragesView } from "../src/views/StoragesView";

function Root() {
  return (
    <div className="h-screen w-screen overflow-hidden bg-ink-1000 text-fg">
      <StoragesView />
    </div>
  );
}

const container = document.getElementById("root");
if (!container) throw new Error("#root missing");
createRoot(container).render(<Root />);
