/* DOM stubs for SSR smoke tests — must be imported FIRST so they exist
   before app modules that touch document/window at load time. */

// @ts-expect-error minimal DOM stub for SSR
globalThis.document = {
  documentElement: { lang: "en" },
  body: {},
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
};
