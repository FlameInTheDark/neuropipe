/* Runs an SSR script that renders the real JsonViewerModal component (TSX +
   "@" path aliases + a createPortal shim so renderToString can exercise portal
   content). esbuild comes in via the vite devDependency; no extra tooling
   needed. The compiled bundle is written OUTSIDE the repo so the working tree
   stays clean.
   
   Run: node scripts/run-jsonviewer-modal-test.cjs [entry.mts]
         (defaults to scripts/test-jsonviewer-modal.mts) */

const { build } = require("esbuild");
const path = require("node:path");
const { pathToFileURL } = require("node:url");

const root = path.resolve(__dirname, "..");

/** Remap the exact specifier "react-dom" to the inline-portal shim while
    leaving "react-dom/server" (renderToString) untouched. The shim itself
    re-exports the real react-dom, so its own import must bypass the shim. */
const exactReactDom = {
  name: "exact-react-dom",
  setup(build) {
    build.onResolve({ filter: /^react-dom$/ }, (args) => {
      if (args.importer && args.importer.includes("react-dom-portal-shim")) return;
      return { path: path.resolve(__dirname, "react-dom-portal-shim.mts") };
    });
  },
};

const outfile = path.resolve("/home/z/my-project/scripts/.jsonviewer-modal-test.cjs");

const entryArg = process.argv[2] ? path.resolve(process.cwd(), process.argv[2]) : path.resolve(__dirname, "test-jsonviewer-modal.mts");

build({
  entryPoints: [entryArg],
  bundle: true,
  platform: "node",
  format: "cjs",
  outfile,
  loader: { ".mts": "ts", ".ts": "ts" },
  jsx: "automatic",
  alias: { "@": path.resolve(root, "src") },
  plugins: [exactReactDom],
  define: { "process.env.NODE_ENV": '"production"' },
  logLevel: "silent",
})
  .then(() => import(pathToFileURL(outfile).href))
  .catch((err) => {
    console.error(err.message || err);
    process.exit(1);
  });
