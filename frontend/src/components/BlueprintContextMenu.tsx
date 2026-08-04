import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { ChevronRight, Copy, Plus, Search, Trash2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip } from "@/components/ui/tooltip";
import { usePersistedCollapsedSections } from "@/lib/preferences";
import { cn } from "@/lib/utils";
import type { NodeDefinition } from "@/lib/types";

export interface BlueprintContextMenuState {
  x: number;
  y: number;
  position: { x: number; y: number };
  nodeID?: string;
  edgeID?: string;
  source?: string;
}

/** The one keyboard-accessible Blueprint menu shared by every graph workspace. */
export function BlueprintContextMenu({
  menu,
  definitions,
  search,
  onSearch,
  onAdd,
  onDuplicate,
  onDelete,
  onClose,
  preferenceKey,
  onRemoveEdge,
  onInsertReroute,
}: {
  menu: BlueprintContextMenuState;
  definitions: NodeDefinition[];
  search: string;
  onSearch: (value: string) => void;
  onAdd: (definition: NodeDefinition) => void;
  onDuplicate: () => void;
  onDelete: () => void;
  onClose: () => void;
  preferenceKey: string;
  onRemoveEdge?: (edgeID: string) => void;
  onInsertReroute?: (edgeID: string, position: { x: number; y: number }) => void;
}) {
  const { t } = useTranslation();
  const [collapsedCategories, toggleCategory] = usePersistedCollapsedSections(
    preferenceKey,
  );
  const groups = useMemo(
    () =>
      definitions.reduce<Record<string, NodeDefinition[]>>(
        (result, definition) => {
          (result[definition.category] ??= []).push(definition);
          return result;
        },
        {},
      ),
    [definitions],
  );
  const searching = search.trim().length > 0;

  if (menu.edgeID && onRemoveEdge) {
    return (
      <div
        role="menu"
        aria-label={t("canvas.connectionOptions")}
        className="absolute z-30 w-52 overflow-hidden rounded-lg border border-zinc-700 bg-zinc-950 p-1 shadow-2xl"
        style={{ left: menu.x, top: menu.y }}
      >
        {onInsertReroute ? (
          <button
            autoFocus
            role="menuitem"
            className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-xs text-zinc-200 hover:bg-zinc-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"
            onClick={() => {
              onInsertReroute(menu.edgeID!, menu.position);
              onClose();
            }}
          >
            <Plus className="size-3.5" />
            {t("canvas.insertReroute")}
          </button>
        ) : null}
        <button
          role="menuitem"
          className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-xs text-red-300 hover:bg-red-500/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-400/50"
          onClick={() => {
            onRemoveEdge(menu.edgeID!);
            onClose();
          }}
        >
          <Trash2 className="size-3.5" />
          {t("canvas.removeConnection")}
        </button>
        {onInsertReroute ? <p className="border-t border-zinc-800 px-2.5 py-2 text-[10px] leading-4 text-zinc-600">{t("canvas.reconnectHint")}</p> : null}
      </div>
    );
  }

  return (
    <div
      role="menu"
      aria-label={t("canvas.options")}
      className="absolute z-30 w-80 overflow-hidden rounded-lg border border-zinc-700 bg-zinc-950 shadow-2xl"
      style={{ left: menu.x, top: menu.y }}
    >
      <div className="flex items-center border-b border-zinc-800 px-2 py-2">
        <Search className="mr-2 size-3.5 text-zinc-600" />
        <input
          autoFocus
          value={search}
          onChange={(event) => onSearch(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Escape") onClose();
          }}
          className="w-full bg-transparent text-xs text-zinc-200 outline-none placeholder:text-zinc-600"
          placeholder={menu.source ? t("canvas.addToWire") : t("canvas.search")}
        />
        <Tooltip content={t("common.close")} side="bottom">
          <button className="rounded p-1 hover:bg-zinc-800" aria-label={t("common.close")} onClick={onClose}>
            <X className="size-3.5 text-zinc-600" />
          </button>
        </Tooltip>
      </div>
      {menu.nodeID ? (
        <div className="flex gap-1 border-b border-zinc-800 p-2">
          <Button size="sm" variant="ghost" onClick={() => { onDuplicate(); onClose(); }}>
            <Copy className="size-3.5" />
            {t("editorActions.duplicate")}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => { onDelete(); onClose(); }}>
            <Trash2 className="size-3.5 text-red-300" />
            {t("editorActions.delete")}
          </Button>
        </div>
      ) : null}
      <div className="muted-scroll max-h-80 overflow-y-auto p-1">
        {Object.entries(groups).map(([category, items]) => {
          const expanded = searching || !collapsedCategories.has(category);
          return (
            <section key={category} className="overflow-hidden rounded-md">
              <button type="button" className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-zinc-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-600" aria-expanded={expanded} onClick={() => toggleCategory(category)}>
                <ChevronRight className={cn("size-3.5 shrink-0 text-zinc-500 transition-transform", expanded && "rotate-90")} />
                <span className="min-w-0 flex-1 text-xs font-medium text-zinc-300">{category}</span>
                <span className="rounded bg-zinc-900 px-1.5 py-0.5 font-mono text-[10px] text-zinc-600">{items.length}</span>
              </button>
              {expanded ? <div className="mb-1 border-l border-zinc-800 pl-2">{items.map((definition) => <button key={definition.type} type="button" onClick={() => onAdd(definition)} className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left hover:bg-zinc-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-600"><div className="flex size-6 shrink-0 items-center justify-center rounded bg-zinc-800"><Plus className="size-3.5 text-zinc-300" /></div><span className="min-w-0 flex-1"><span className="block truncate text-xs font-medium text-zinc-200">{definition.label}</span><span className="block truncate text-[10px] text-zinc-600">{definition.description}</span></span></button>)}</div> : null}
            </section>
          );
        })}
        {definitions.length === 0 ? <p className="px-3 py-5 text-center text-xs text-zinc-600">{t("library.noMatches")}</p> : null}
      </div>
      <div className="border-t border-zinc-800 px-3 py-2 text-[10px] text-zinc-600">
        {t("canvas.hint")}
      </div>
    </div>
  );
}
