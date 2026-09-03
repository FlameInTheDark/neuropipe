/* extractPayload replica so live harnesses bundle only this stub instead of
   the whole App module. Keep in sync with frontend/src/App.tsx. */
export function extractPayload(event: unknown): unknown {
  if (event && typeof event === "object" && "data" in event) {
    return (event as { data?: unknown }).data ?? event;
  }
  return event;
}
