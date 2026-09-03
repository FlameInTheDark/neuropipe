/**
 * Minimal @wailsio/runtime stand-in for live harnesses: implements the Events
 * surface ChatView uses (On) and routes emissions through a global registry
 * the bridge mock and the Playwright driver share.
 */
const harness = window as unknown as {
  __wailsEvents: Map<string, Array<(data?: unknown) => void>>;
};

if (!harness.__wailsEvents) harness.__wailsEvents = new Map();

function on(event: string, callback: (data?: unknown) => void): () => void {
  const list = harness.__wailsEvents.get(event) ?? [];
  list.push(callback);
  harness.__wailsEvents.set(event, list);
  return () => {
    const current = harness.__wailsEvents.get(event) ?? [];
    const index = current.indexOf(callback);
    if (index >= 0) current.splice(index, 1);
  };
}

export const Events = { On: on, Off: () => {}, Emit: () => {} };
export const Window = { Name: () => "main" };
export const runtime = {};
