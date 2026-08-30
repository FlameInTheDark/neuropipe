import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import "./i18n";
import App from "./App";
import { installDefaultContextMenuPolicy } from "./lib/context-menu";
import { initTheme } from "./stores/theme";

/* release builds show only Neuropipe's own context menus — the native browser
   menu is suppressed app-wide (dev builds keep it for Inspect Element etc.).
   Installed before anything renders so no element ever slips through. */
installDefaultContextMenuPolicy(import.meta.env.DEV);

/* paint with the persisted theme before the first React render */
initTheme();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>
);
