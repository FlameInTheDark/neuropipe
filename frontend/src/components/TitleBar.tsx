import { useCallback, useEffect, useState } from "react";
import { Download, Maximize2, Minus, Square, X } from "lucide-react";
import { Application, Events, Window } from "@wailsio/runtime";
import appIcon from "@/assets/appicon.png";
import { Tooltip } from "@/components/ui/tooltip";
import { useUIStore } from "@/stores/ui";
import { useTranslation } from 'react-i18next';
import { desktop } from "@/lib/bridge";
import type { UpdateAvailability } from "@/lib/types";

export function TitleBar() {
  const [maximised, setMaximised] = useState(false);
  const [update, setUpdate] = useState<UpdateAvailability>();
  const { sidebarCollapsed, toggleSidebar } = useUIStore();
  const { t } = useTranslation();

  const syncMaximised = useCallback(() => {
    void Window.IsMaximised()
      .then(setMaximised)
      .catch(() => setMaximised(false));
  }, []);

  useEffect(() => {
    syncMaximised();
  }, [syncMaximised]);

  useEffect(() => {
    let active = true;
    void desktop.getUpdateAvailability().then((value) => {
      if (active && value.available) setUpdate(value);
    }).catch(() => undefined);
    const stop = Events.On("app.update.available", (event) => {
      const value = (event?.data ?? event) as UpdateAvailability | undefined;
      if (value?.available) setUpdate(value);
    });
    return () => { active = false; stop(); };
  }, []);

  const toggleMaximise = () => {
    void Window.ToggleMaximise();
    window.setTimeout(syncMaximised, 80);
  };

  return (
    <header className="app-titlebar title-drag">
      <div className="flex items-center gap-2 px-3 text-zinc-300">
        <Tooltip content={sidebarCollapsed ? t('titlebar.expandSidebar') : t('titlebar.collapseSidebar')} side="bottom" align="start">
          <button
            type="button"
            onClick={toggleSidebar}
            className="title-no-drag flex size-7 items-center justify-center rounded-md transition-colors hover:bg-zinc-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"
            aria-label={sidebarCollapsed ? t('titlebar.expandSidebar') : t('titlebar.collapseSidebar')}
            aria-expanded={!sidebarCollapsed}
          >
            <img src={appIcon} alt="" className="size-5 rounded-[5px]" />
          </button>
        </Tooltip>
        <span className="select-none text-xs font-semibold tracking-tight">
          Neuropipe
        </span>
      </div>
      <span aria-hidden="true" />
      <div className="title-no-drag flex h-full justify-self-end">
        {update?.available && update.version ? <Tooltip content={t('titlebar.openUpdate', { version: update.version })} side="bottom">
          <button type="button" className="titlebar-update" onClick={() => { void desktop.openUpdateRelease().catch(() => undefined); }} aria-label={t('titlebar.openUpdate', { version: update.version })}>
            <Download className="size-3.5" />
            <span>{t('titlebar.updateAvailable', { version: update.version })}</span>
          </button>
        </Tooltip> : null}
        <button type="button" className="window-control" onClick={() => { void Window.Minimise(); }} aria-label={t('titlebar.minimise')}>
          <Minus className="size-4" strokeWidth={2} />
        </button>
        <button
          type="button"
          className="window-control"
          onClick={toggleMaximise}
          aria-label={maximised ? t('titlebar.restore') : t('titlebar.maximise')}
        >
          {maximised ? <Square className="size-3" /> : <Maximize2 className="size-3.5" />}
        </button>
        <button type="button" className="window-control window-control-close" onClick={() => { void Application.Quit(); }} aria-label={t('titlebar.close')}>
          <X className="size-4" />
        </button>
      </div>
    </header>
  );
}
