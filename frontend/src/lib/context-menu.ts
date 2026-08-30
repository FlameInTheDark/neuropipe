/**
 * App-wide default-context-menu policy.
 *
 * The webview still shows the browser's native context menu wherever no
 * Neuropipe menu is attached (plain labels, inputs, panes without an
 * onContextMenu handler). The release app should only ever show our own
 * styled menus, so production builds suppress the native menu globally: one
 * bubble-phase `contextmenu` listener on `document` calls preventDefault.
 *
 * Custom menus keep working untouched in both builds — ContextMenu.tsx
 * renders its own portal UI and already preventDefaults itself, and
 * preventDefault never stops propagation, so every onContextMenu handler
 * still fires. The listener deliberately does NOT use capture: document is
 * the last stop of the bubble order, so element-level handlers (React's root
 * container sits below document) always observe a fresh, unconsumed event.
 *
 * Dev builds keep the native menu (Inspect Element and friends while
 * developing). `dev` is passed in by main.tsx (`import.meta.env.DEV`) so the
 * policy stays a pure function of its inputs; `target` is injectable for the
 * unit suite (scripts/test-context-menu.mts). Installed exactly once at
 * startup, before the first React render.
 */
export function installDefaultContextMenuPolicy(dev: boolean, target: Document = document): void {
  if (dev) return;
  target.addEventListener("contextmenu", (e) => e.preventDefault());
}
