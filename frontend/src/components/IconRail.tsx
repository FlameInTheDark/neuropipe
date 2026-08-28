import { useTranslation } from "react-i18next";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";
import { cn } from "../utils/cn";

export const NAV_TOP = [
  { id: "board", icon: "Grid2x2", labelKey: "nav.buttonBoard" },
  { id: "chat", icon: "MessagesSquare", labelKey: "nav.chat" },
  { id: "triggers", icon: "CircleDot", labelKey: "nav.triggers" },
  { id: "schedules", icon: "Clock", labelKey: "nav.schedules" },
  { id: "reports", icon: "FileText", labelKey: "nav.reports" },
];

export const NAV_MIDDLE = [
  { id: "pipelines", icon: "Cable", labelKey: "nav.pipelines" },
  { id: "functions", icon: "SquareFunction", labelKey: "nav.functions" },
  { id: "variables", icon: "Braces", labelKey: "nav.variables" },
  { id: "databases", icon: "Database", labelKey: "nav.databases" },
];

export const NAV_BOTTOM = [
  { id: "metrics", icon: "Activity", labelKey: "nav.metrics" },
  { id: "docs", icon: "BookOpen", labelKey: "nav.documentation" },
  { id: "settings", icon: "Settings2", labelKey: "nav.settings" },
];

export const NAV_ALL = [...NAV_TOP, ...NAV_MIDDLE, ...NAV_BOTTOM];

/* geometry — the icon column is exactly the collapsed content width,
   so icons occupy the same pixel position in both states and never move. */
const RAIL_COLLAPSED = 52;
const RAIL_EXPANDED = 208;
const PAD = 8;
const ICON_BOX = RAIL_COLLAPSED - PAD * 2; // 36
const LABEL_W = RAIL_EXPANDED - PAD * 2 - ICON_BOX; // 156

const EASE = "cubic-bezier(0.4, 0, 0.2, 1)";

function Label({ expanded, children }: { expanded: boolean; children: React.ReactNode }) {
  return (
    <span
      aria-hidden={!expanded}
      style={{
        width: expanded ? LABEL_W : 0,
        transition: `width 220ms ${EASE}, opacity 160ms ${EASE}`,
        transitionDelay: expanded ? "30ms, 60ms" : "0ms, 0ms",
      }}
      className={cn(
        "flex items-center overflow-hidden whitespace-nowrap",
        expanded ? "opacity-100" : "opacity-0",
      )}
    >
      {children}
    </span>
  );
}

function RailButton({
  item,
  active,
  expanded,
  onClick,
}: {
  item: { id: string; icon: string; labelKey: string };
  active: boolean;
  expanded: boolean;
  onClick: () => void;
}) {
  const { t } = useTranslation();
  return (
    <Tooltip content={t(item.labelKey)} side="right" disabled={expanded} className="w-full">      <button
        onClick={onClick}
        className={cn(
          "group relative flex h-9 w-full shrink-0 items-center rounded-lg transition-colors duration-150",
          active ? "bg-ink-750 text-fg" : "text-fg-subtle hover:bg-ink-850 hover:text-fg",
        )}
      >
        {active && <span className="absolute left-0 h-4 w-[2px] rounded-r bg-ink-50" />}

        <span style={{ width: ICON_BOX }} className="grid shrink-0 place-items-center">
          <Icon name={item.icon} className="h-[17px] w-[17px]" />
        </span>

        <Label expanded={expanded}>
          <span className="truncate pr-3 text-[12.5px] font-medium">{t(item.labelKey)}</span>
        </Label>
      </button>
    </Tooltip>
  );
}

export function IconRail({
  active,
  expanded,
  onSelect,
  onToggle,
}: {
  active: string;
  expanded: boolean;
  onSelect: (id: string) => void;
  onToggle: () => void;
}) {
  return (
    <nav
      style={{
        width: expanded ? RAIL_EXPANDED : RAIL_COLLAPSED,
        padding: PAD,
        transition: `width 220ms ${EASE}`,
      }}
      className="absolute top-3 bottom-3 left-3 z-40 flex flex-col gap-1 rounded-xl border border-ink-700 bg-ink-900/90 shadow-[0_24px_60px_-20px_rgba(0,0,0,0.95)] backdrop-blur-xl"
    >
      {/* brand / toggle */}
      <Tooltip content="Expand sidebar" side="right" disabled={expanded} className="w-full shrink-0">
      <button
        onClick={onToggle}
        className="group relative flex h-9 w-full shrink-0 items-center rounded-lg transition-colors duration-150 hover:bg-ink-850"
      >
        <span style={{ width: ICON_BOX }} className="grid shrink-0 place-items-center">
          <span className="grid h-7 w-7 place-items-center rounded-md bg-ink-50 text-[13px] font-semibold text-fg-onEmphasis">
            N
          </span>
        </span>

        <Label expanded={expanded}>
          <span className="truncate text-[13px] font-semibold text-fg">Neuropipe</span>
          <Icon
            name="PanelLeft"
            className="mr-2.5 ml-auto h-4 w-4 shrink-0 text-fg-faint transition-colors group-hover:text-fg-muted"
          />
        </Label>
      </button>
      </Tooltip>

      <div className="my-1 h-px w-full shrink-0 bg-ink-750" />

      <div className="flex min-h-0 flex-1 flex-col gap-1 overflow-x-hidden overflow-y-auto">
        {NAV_TOP.map((i) => (
          <RailButton key={i.id} item={i} active={active === i.id} expanded={expanded} onClick={() => onSelect(i.id)} />
        ))}
        <div className="my-1 h-px w-full shrink-0 bg-ink-750" />
        {NAV_MIDDLE.map((i) => (
          <RailButton key={i.id} item={i} active={active === i.id} expanded={expanded} onClick={() => onSelect(i.id)} />
        ))}
      </div>

      <div className="mt-1 flex shrink-0 flex-col gap-1">
        <div className="mb-1 h-px w-full bg-ink-750" />
        {NAV_BOTTOM.map((i) => (
          <RailButton key={i.id} item={i} active={active === i.id} expanded={expanded} onClick={() => onSelect(i.id)} />
        ))}
      </div>
    </nav>
  );
}
