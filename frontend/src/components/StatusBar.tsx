import { useTranslation } from "react-i18next";
import { desktop } from "@/lib/bridge";
import type { UpdateAvailability } from "@/lib/types";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";
import { cn } from "../utils/cn";

function Item({
  icon,
  children,
  className,
  onClick,
  title,
  spin,
}: {
  icon?: string;
  children: React.ReactNode;
  className?: string;
  onClick?: () => void;
  title?: string;
  spin?: boolean;
}) {
  const button = (
    <button
      onClick={onClick}
      aria-label={title}
      className={cn(
        "flex h-full items-center gap-1.5 px-2 text-[11px] text-fg-subtle transition hover:bg-ink-850 hover:text-fg",
        !onClick && "cursor-default hover:bg-transparent hover:text-fg-subtle",
        className,
      )}
    >
      {icon && <Icon name={icon} className={cn("h-3 w-3", spin && "animate-spin")} />}
      {children}
    </button>
  );
  return title ? (
    <Tooltip content={title} side="top" delay={400} className="flex">
      {button}
    </Tooltip>
  ) : (
    button
  );
}

export function StatusBar({
  inEditor,
  nodes,
  edges,
  zoom,
  snap,
  running,
  saved,
  selected,
  activeRuns,
  contentDirectory,
  update,
  onFit,
}: {
  inEditor: boolean;
  nodes: number;
  edges: number;
  zoom: number;
  snap: boolean;
  running: boolean;
  saved: string | null;
  selected: string | null;
  activeRuns: number;
  contentDirectory?: string;
  update?: UpdateAvailability | null;
  onFit: () => void;
}) {
  const { t } = useTranslation();
  return (
    <footer className="flex h-[26px] shrink-0 items-center border-t border-seam bg-ink-950 px-1 select-none">
      {inEditor ? (
        <>
          <Item
            icon={running ? "Loader2" : "Check"}
            spin={running}
            className={running ? "text-fg" : "text-success-fg/80"}
          >
            {running ? t("editor.executingDraft") : t("editor.graphValid")}
          </Item>
          <span className="h-3 w-px bg-ink-750" />
          <Item icon="Boxes">{t("status.nodes", { count: nodes })}</Item>
          <Item icon="Cable">{t("status.links", { count: edges })}</Item>
          {selected && (
            <>
              <span className="h-3 w-px bg-ink-750" />
              <Item icon="Crosshair">
                <span className="font-mono text-fg-muted">{selected}</span>
              </Item>
            </>
          )}
          <div className="ml-auto flex h-full items-center">
            <Item icon="Magnet" className={snap ? "text-fg" : undefined}>
              {snap ? t("status.snapOn") : t("status.snapOff")}
            </Item>
            <Item icon="Maximize2" onClick={onFit} title={t("editor.fitGraph")}>
              {Math.round(zoom * 100)}%
            </Item>
            {saved && (
              <>
                <span className="h-3 w-px bg-ink-750" />
                <Item icon="Clock">{t("status.saved", { time: saved })}</Item>
              </>
            )}
            <Item icon="Command">⌘K</Item>
          </div>
        </>
      ) : (
        <>
          <Item icon="Zap" className="text-success-fg/80">
            {t("status.runtimeReady")}
          </Item>
          <span className="h-3 w-px bg-ink-750" />
          <Item icon="Activity">
            {t("status.activeRuns", { count: activeRuns })}
          </Item>
          <div className="ml-auto flex h-full items-center">
            <Item icon="HardDrive">{contentDirectory || t("status.local")}</Item>
            {update?.available && (
              <Tooltip content={t("titlebar.openUpdate", { version: update.version ?? "" })} side="top">
                <button
                  onClick={() => void desktop.openUpdateRelease().catch(() => undefined)}
                  aria-label={t("titlebar.updateAvailable", { version: update.version ?? "" })}
                  className="flex h-full items-center gap-1.5 px-2 text-[11px] font-medium text-success-fg transition hover:bg-success/10 hover:text-success-fg"
                >
                  <Icon name="Download" className="h-3 w-3" />
                  {t("status.updateAvailable")}
                </button>
              </Tooltip>
            )}
            <Item icon="Command">⌘K</Item>
          </div>
        </>
      )}
    </footer>
  );
}
