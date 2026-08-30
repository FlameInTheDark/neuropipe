import path from "path";
import { fileURLToPath } from "url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { viteSingleFile } from "vite-plugin-singlefile";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Wails v3 dev mode sets WAILS_VITE_PORT (default 9245) and
// FRONTEND_DEVSERVER_URL so the Go asset server can proxy to Vite.
// Use that port when present; fall back to Vite's default 5173 for
// standalone `npm run dev` outside Wails.
const wailsPort = process.env.WAILS_VITE_PORT;
const port = wailsPort ? parseInt(wailsPort, 10) : 5173;

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), viteSingleFile()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  server: {
    // Wails v3's dev asset proxy force-dials IPv4 (tcp4) when the dev server
    // host is localhost/127.0.0.1 (wails v3 internal/assetserver/build_dev.go,
    // "Force IPv4 for localhost connections to avoid IPv6 issues on Windows").
    // Vite's default `localhost` host can instead bind IPv6-only ([::1]) on
    // Windows with Node >= 17, leaving 127.0.0.1 refused and the app window
    // blank with "[ExternalAssetHandler] Proxy error ... connectex" while the
    // browser still loads fine. Pin IPv4 loopback so both sides always meet.
    host: "127.0.0.1",
    port,
    strictPort: true,
  },
  build: { target: "es2022", sourcemap: true },
});
