import { Clipboard } from "@wailsio/runtime";

/**
 * Copies text through the strongest available path:
 *  1. the native Wails clipboard bridge (always works inside the desktop
 *     webview, where the browser clipboard API is frequently blocked), then
 *  2. the async web clipboard API, then
 *  3. the legacy select-an-offscreen-textarea + execCommand trick.
 *
 * Returns false when every path fails so callers can surface a dead copy
 * instead of silently doing nothing.
 */
export async function copyText(text: string): Promise<boolean> {
  try {
    await Clipboard.SetText(text);
    return true;
  } catch {
    /* not running under the Wails runtime, or the bridge is unavailable */
  }
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    /* permission denied — fall through to the legacy path */
  }
  try {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "true");
    area.style.position = "fixed";
    area.style.top = "-1000px";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.focus();
    area.select();
    const ok = document.execCommand("copy");
    area.remove();
    return ok;
  } catch {
    return false;
  }
}
