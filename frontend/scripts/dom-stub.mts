/* DOM stubs for SSR smoke tests — must be imported FIRST so they exist
   before app modules that touch document/window at load time. */

// @ts-expect-error minimal DOM stub for SSR
globalThis.document = {
  documentElement: { lang: "en" },
  // nodeType 1 = ELEMENT_NODE: react-dom 19.2's createPortal validates the
  // container eagerly (even inside renderToString), so the body stub has to
  // look like a real element or every portal-owning component throws
  body: { nodeType: 1 },
  createElement: () => ({
    style: {},
    setAttribute() {},
    appendChild() {},
    remove() {},
  }),
  addEventListener() {},
  removeEventListener() {},
};
// @ts-expect-error minimal window stub (listeners/timers)
globalThis.window = {
  addEventListener() {},
  removeEventListener() {},
  setTimeout,
  clearTimeout,
  setInterval,
  clearInterval,
};

// @ts-expect-error MouseEvent stub — @wailsio/runtime probes it at import
// time (canTrackButtons) even when no call is ever made against Wails.
class MouseEventStub {
  buttons = 0;
  constructor(public type: string, public init: Record<string, unknown> = {}) {}
}
globalThis.MouseEvent = MouseEventStub;
