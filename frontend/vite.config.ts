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
    port,
    strictPort: true,
  },
  build: { target: "es2022", sourcemap: true },
});
