import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { LibraryCategory, LibraryItem } from "@/types";
import { Icon } from "./icons";
import { Empty, PanelHeader } from "./ui";
import { cn } from "../utils/cn";

const DEFAULT_OPEN = new Set(["AI", "Actions", "Chat"]);

export function LibraryPanel({
  library,
  onAdd,
}: {
  library: LibraryCategory[];
  onAdd: (item: LibraryItem, group: string) => void;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return library;
    return library
      .map((c) => ({
        ...c,
        items: c.items.filter(
          (i) => i.name.toLowerCase().includes(q) || i.desc.toLowerCase().includes(q),
        ),
      }))
      .filter((c) => c.items.length > 0);
  }, [query, library]);

  const searching = query.trim().length > 0;
  const total = library.reduce((a, c) => a + c.count, 0);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <PanelHeader
        title={t("library.nodes")}
        icon="Boxes"
        right={<span className="font-mono text-[10px] text-fg-faint">{total}</span>}
      />

      <div className="border-b border-seam p-2">
        <div className="group relative flex h-8 items-center gap-2 rounded-md border border-ink-700/70 bg-ink-850 px-2.5 transition focus-within:border-ink-500 focus-within:bg-ink-800">
          <Icon name="Search" className="h-3.5 w-3.5 shrink-0 text-fg-faint" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("library.search")}
            aria-label={t("library.search")}
            className="min-w-0 flex-1 bg-transparent text-[12.5px] text-fg placeholder:text-fg-faint"
          />
          {query && (
            <button onClick={() => setQuery("")} aria-label={t("common.clear")} className="text-fg-faint hover:text-fg-muted">
              <Icon name="X" className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain">
        {filtered.length === 0 && <Empty icon="Search" text={t("library.noMatches")} />}

        {filtered.map((cat) => {
          const expanded = searching || (collapsed[cat.name] ?? !DEFAULT_OPEN.has(cat.name)) === false;
          return (
            <div key={cat.name} className="border-b border-seam/70 last:border-b-0">
              <button
                onClick={() => setCollapsed((m) => ({ ...m, [cat.name]: expanded }))}
                className="flex w-full items-center gap-1.5 px-2.5 py-[7px] text-left transition hover:bg-ink-850"
              >
                <Icon
                  name="ChevronRight"
                  className={cn(
                    "h-3 w-3 text-fg-faint transition-transform",
                    expanded && "rotate-90 text-fg-subtle",
                  )}
                />
                <span className="text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">
                  {cat.name}
                </span>
                <span className="ml-auto rounded bg-ink-800 px-1.5 font-mono text-[10px] text-fg-faint">
                  {cat.count}
                </span>
              </button>

              {expanded && (
                <ul className="pb-1">
                  {cat.items.map((item) => (
                    <li key={`${cat.name}-${item.type ?? item.functionId ?? item.name}`}>
                      <button
                        draggable
                        onDragStart={(e) => {
                          e.dataTransfer.effectAllowed = "copy";
                          e.dataTransfer.setData(
                            "application/x-neuropipe-node",
                            JSON.stringify({ item, category: cat.name }),
                          );
                          // lightweight drag image so the cursor stays readable
                          const ghost = document.createElement("div");
                          ghost.textContent = item.name;
                          ghost.style.cssText =
                            "position:fixed;top:-1000px;left:-1000px;padding:6px 10px;border-radius:8px;" +
                            "border:1px solid var(--ink-600);background:var(--ink-750);color:var(--fg);" +
                            "font:500 12.5px Inter,sans-serif;white-space:nowrap;";
                          document.body.appendChild(ghost);
                          e.dataTransfer.setDragImage(ghost, 12, 14);
                          window.setTimeout(() => ghost.remove(), 0);
                        }}
                        onClick={() => onAdd(item, cat.name)}
                        className="group flex w-full cursor-grab items-start gap-2.5 px-2.5 py-1.5 pl-[26px] text-left transition hover:bg-ink-800 active:cursor-grabbing"
                      >
                        <span className="mt-[1px] grid h-6 w-6 shrink-0 place-items-center rounded-md border border-ink-700 bg-ink-850 text-fg-subtle transition group-hover:border-ink-600 group-hover:bg-ink-750 group-hover:text-fg">
                          <Icon name={item.icon} className="h-3.5 w-3.5" />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="flex items-center gap-1.5">
                            <span className="truncate text-[12.5px] font-medium text-fg">
                              {item.name}
                            </span>
                            <Icon
                              name="Plus"
                              className="h-3 w-3 shrink-0 text-fg-faint opacity-0 transition group-hover:opacity-100"
                            />
                          </span>
                          <span className="mt-[1px] line-clamp-2 block text-[11.5px] leading-[1.45] text-fg-subtle">
                            {item.desc}
                          </span>
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          );
        })}
      </div>

      <div className="flex h-8 shrink-0 items-center gap-1.5 border-t border-seam px-2.5 text-[11px] text-fg-faint">
        <Icon name="MousePointer2" className="h-3 w-3" />
        {t("library.hint")}
      </div>
    </div>
  );
}
