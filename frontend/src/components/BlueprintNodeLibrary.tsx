import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { ChevronRight, GripVertical, Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { usePersistedCollapsedSections } from "@/lib/preferences";
import { cn } from "@/lib/utils";
import type { NodeDefinition } from "@/lib/types";

/** Shared foldable node library used by Blueprint pipeline and function graphs. */
export function BlueprintNodeLibrary({
  definitions,
  search,
  onSearch,
  onAdd,
  dragMime,
  preferenceKey,
}: {
  definitions: NodeDefinition[];
  search: string;
  onSearch: (value: string) => void;
  onAdd: (definition: NodeDefinition) => void;
  dragMime: string;
  preferenceKey: string;
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
  return (
    <aside className="muted-scroll min-h-0 overflow-y-auto border-r border-zinc-800 bg-zinc-950">
      <div className="sticky top-0 z-10 border-b border-zinc-800 bg-zinc-950 p-3">
        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-2 size-3.5 text-zinc-600" />
          <Input
            value={search}
            onChange={(event) => onSearch(event.target.value)}
            className="pl-8 text-xs"
            placeholder={t("library.search")}
          />
        </div>
      </div>
      <div className="p-2">
        {Object.entries(groups).map(([category, items]) => {
          const expanded = searching || !collapsedCategories.has(category);
          return (
            <section key={category} className="mb-1 overflow-hidden rounded-md">
              <button
                type="button"
                className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left hover:bg-zinc-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-600"
                aria-expanded={expanded}
                onClick={() => toggleCategory(category)}
              >
                <ChevronRight
                  className={cn(
                    "size-3.5 shrink-0 text-zinc-500 transition-transform",
                    expanded && "rotate-90",
                  )}
                />
                <span className="min-w-0 flex-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-600">
                  {category}
                </span>
                <span className="rounded bg-zinc-900 px-1.5 py-0.5 font-mono text-[10px] text-zinc-600">
                  {items.length}
                </span>
              </button>
              {expanded ? (
                <div className="mb-2 border-l border-zinc-800 pl-2">
                  {items.map((definition) => (
                    <button
                      key={definition.type}
                      type="button"
                      draggable
                      onDragStart={(event) =>
                        event.dataTransfer.setData(dragMime, definition.type)
                      }
                      onClick={() => onAdd(definition)}
                      className="flex w-full items-start gap-2 rounded-md px-2 py-2 text-left hover:bg-zinc-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-600"
                    >
                      <GripVertical className="mt-0.5 size-3 shrink-0 text-zinc-700" />
                      <span className="min-w-0">
                        <span className="block text-xs font-medium text-zinc-300">
                          {definition.label}
                        </span>
                        <span className="mt-0.5 block line-clamp-2 text-[11px] leading-4 text-zinc-600">
                          {definition.description}
                        </span>
                      </span>
                    </button>
                  ))}
                </div>
              ) : null}
            </section>
          );
        })}
        {definitions.length === 0 ? (
          <p className="px-3 py-5 text-center text-xs text-zinc-600">
            {t("library.noMatches")}
          </p>
        ) : null}
      </div>
    </aside>
  );
}
