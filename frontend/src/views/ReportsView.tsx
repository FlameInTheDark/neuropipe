import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { Report } from "@/lib/types";
import type { Workspace } from "@/features/workspace/useWorkspace";
import type { NavApi } from "@/features/workspace/useWorkspaceNav";
import { ask } from "@/stores/confirmation";
import { SearchInput, ViewShell, EmptyState } from "../components/ViewShell";
import { MarkdownRenderer } from "../components/MarkdownRenderer";
import { Button } from "../components/ui";
import { Icon } from "../components/icons";
import { Dropdown } from "../components/Dropdown";
import { DateRangePicker } from "../components/primitives/DateRangePicker";
import { useCtxMenu } from "../components/ContextMenu";
import { cn } from "../utils/cn";

const SORTS = [
  { value: "newest", label: "reports.newest", icon: "Clock" },
  { value: "oldest", label: "reports.oldest", icon: "History" },
  { value: "tag", label: "reports.sortTag", icon: "Tags" },
];

/** Boundary timestamp for a range value: accepts "YYYY-MM-DD" (day start or
 *  full-day end) and "YYYY-MM-DDTHH:mm" (exact time). */
function rangeBoundary(value: string, endOfDay = false): number | undefined {
  if (!/^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2})?$/.test(value)) return undefined;
  if (value.includes("T")) {
    const time = Date.parse(value);
    return Number.isNaN(time) ? undefined : time;
  }
  const time = Date.parse(`${value}T${endOfDay ? "23:59:59.999" : "00:00:00"}`);
  return Number.isNaN(time) ? undefined : time;
}

function excerpt(markdown: string, max = 180): string {
  const flat = markdown
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/[#>*_`~\[\]()!-]/g, " ")
    .replace(/https?:\/\/\S+/g, "")
    .replace(/\s+/g, " ")
    .trim();
  return flat.length > max ? `${flat.slice(0, max)}…` : flat;
}

export function ReportsView({ workspace, nav }: { workspace: Workspace; nav: NavApi }) {
  const { t, i18n } = useTranslation();
  const reports = workspace.reports;
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [q, setQ] = useState("");
  const [sort, setSort] = useState("newest");
  /** every clicked tag lands here and shows as a removable chip above the list */
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const ctx = useCtxMenu();

  const isTagSelected = useCallback(
    (tag: string) => selectedTags.some((s) => s.toLowerCase() === tag.toLowerCase()),
    [selectedTags],
  );

  /** Clicking a tag anywhere adds it to the top filters; clicking again removes it. */
  const toggleTag = (tag: string) =>
    setSelectedTags((cur) =>
      cur.some((s) => s.toLowerCase() === tag.toLowerCase())
        ? cur.filter((s) => s.toLowerCase() !== tag.toLowerCase())
        : [...cur, tag],
    );

  /** Every distinct tag across reports, original casing, alphabetical. */
  const availableTags = useMemo(
    () =>
      Array.from(
        new Map(reports.flatMap((r) => r.tags.map((tag) => [tag.toLowerCase(), tag] as const))).values(),
      ).sort((a, b) => a.localeCompare(b)),
    [reports],
  );

  const filtersActive = q.trim() !== "" || selectedTags.length > 0 || from !== "" || to !== "";

  const clearFilters = () => {
    setQ("");
    setSelectedTags([]);
    setFrom("");
    setTo("");
  };

  const filtered = useMemo(() => {
    const query = q.trim().toLowerCase();
    const fromTime = rangeBoundary(from);
    const toTime = rangeBoundary(to, true);
    const tagKeys = selectedTags.map((s) => s.toLowerCase());
    const hits = reports.filter((r) => {
      if (
        query &&
        !r.title.toLowerCase().includes(query) &&
        !r.pipelineName.toLowerCase().includes(query) &&
        !r.tags.some((tag) => tag.toLowerCase().includes(query))
      ) {
        return false;
      }
      /* OR semantics: a report matches when it carries any selected tag */
      if (tagKeys.length > 0 && !r.tags.some((tag) => tagKeys.includes(tag.toLowerCase()))) return false;
      const created = Date.parse(r.createdAt);
      if (fromTime !== undefined && created < fromTime) return false;
      if (toTime !== undefined && created > toTime) return false;
      return true;
    });
    if (sort === "tag") {
      const firstTag = (r: Report) => (r.tags[0] ?? t("reports.uncategorized")).toLowerCase();
      return hits.sort((a, b) => firstTag(a).localeCompare(firstTag(b)) || Date.parse(b.createdAt) - Date.parse(a.createdAt));
    }
    return hits.sort((a, b) =>
      sort === "oldest" ? Date.parse(a.createdAt) - Date.parse(b.createdAt) : Date.parse(b.createdAt) - Date.parse(a.createdAt),
    );
  }, [reports, q, sort, selectedTags, from, to, t]);

  const active = filtered.find((r) => r.id === selectedId) ?? filtered[0];

  useEffect(() => {
    if (!selectedId && filtered[0]) setSelectedId(filtered[0].id);
  }, [filtered, selectedId]);

  const reportMenu = (e: React.MouseEvent, r: Report) => {
    setSelectedId(r.id);
    ctx(e, [
      {
        label: t("common.copy"),
        icon: "Copy",
        onSelect: () => navigator.clipboard?.writeText(r.markdown),
      },
      { type: "sep" as const },
      {
        label: t("reports.openPipelines"),
        icon: "Cable",
        onSelect: () => nav.goto("pipelines"),
      },
      { type: "sep" as const },
      {
        label: t("common.delete"),
        icon: "Trash2",
        danger: true,
        onSelect: async () => {
          const ok = await ask({
            title: t("reports.deleteTitle"),
            description: t("reports.deleteDescription", { name: r.title }),
            confirmLabel: t("reports.deleteConfirm"),
            danger: true,
          });
          if (!ok) return;
          await workspace.deleteReport(r.id);
        },
      },
    ]);
  };

  const dateFmt = new Intl.DateTimeFormat(i18n.resolvedLanguage ?? "en", {
    dateStyle: "medium",
    timeStyle: "short",
  });

  /** Opens the pipeline that produced a report; falls back to a minimal
   *  summary when the pipeline is no longer in the workspace list. */
  const openReportPipeline = (report: Report) => {
    const summary = workspace.pipelines.find((p) => p.id === report.pipelineId);
    void nav.openPipeline(
      summary ?? {
        id: report.pipelineId,
        name: report.pipelineName,
        desc: "",
        icon: "Workflow",
        status: "published",
        version: "",
        triggers: 0,
        updated: "",
      },
    );
  };

  return (
    <ViewShell
      title={t("reports.title")}
      subtitle={t("status.count", { count: filtersActive ? filtered.length : reports.length })}
      padded={false}
    >
      {/* fixed-height column: toolbar/filters stay put; only the list and
          the article body scroll inside their own panes */}
      <div className="flex h-full min-h-0 flex-col">
        <div className="shrink-0 border-b border-seam px-4 py-2">
          <div className="flex items-center gap-2">
            <SearchInput
              value={q}
              onChange={setQ}
              placeholder={t("reports.search")}
              className="w-[220px]"
            />
            <Dropdown
              value={sort}
              onChange={setSort}
              className="w-[150px]"
              options={SORTS.map((s) => ({ ...s, label: t(s.label) }))}
            />
            <DateRangePicker
              withTime
              value={{ from, to }}
              onChange={({ from, to }) => {
                setFrom(from);
                setTo(to);
              }}
            />
            {filtersActive && (
              <Button variant="ghost" icon="X" onClick={clearFilters}>
                {t("common.clear")}
              </Button>
            )}
          </div>
          {(availableTags.length > 0 || selectedTags.length > 0) && (
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <Dropdown
                value=""
                onChange={(tag) => tag && toggleTag(tag)}
                className="w-[170px]"
                placeholder={t("reports.tagAll")}
                options={[
                  { value: "", label: t("reports.tagAll"), icon: "Tags" },
                  ...availableTags.map((tag) => ({
                    value: tag,
                    label: `#${tag}`,
                    icon: isTagSelected(tag) ? "Check" : undefined,
                  })),
                ]}
              />
              {selectedTags.map((tag) => (
                <button
                  key={`filter-${tag.toLowerCase()}`}
                  onClick={() => toggleTag(tag)}
                  aria-label={`${t("common.clear")}: #${tag}`}
                  className="flex items-center gap-1 rounded-md border border-info/40 bg-info/15 px-2 py-1 font-mono text-[11px] text-info-fg transition hover:bg-info/25"
                >
                  #{tag}
                  <Icon name="X" className="h-3 w-3" />
                </button>
              ))}
            </div>
          )}
        </div>

      {filtered.length === 0 ? (
        <div className="grid flex-1 place-items-center overflow-hidden">
          <EmptyState icon="FileText" title={t("reports.emptyTitle")} hint={t("reports.emptyDescription")} />
        </div>
      ) : (
        <div className="flex min-h-0 flex-1">
          <div className="w-[300px] shrink-0 overflow-y-auto border-r border-seam p-2.5">
            {filtered.map((r) => (
              <div
                key={r.id}
                role="button"
                tabIndex={0}
                onClick={() => setSelectedId(r.id)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    setSelectedId(r.id);
                  }
                }}
                onContextMenu={(e) => reportMenu(e, r)}
                className={cn(
                  "mb-2 block w-full cursor-pointer rounded-xl border p-3 text-left transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring/60",
                  active?.id === r.id
                    ? "border-ink-500 bg-ink-800/70"
                    : "border-ink-700/80 bg-ink-850/50 hover:border-ink-600 hover:bg-ink-850",
                )}
              >
                <span className="block truncate text-[12.5px] font-semibold text-fg">{r.title}</span>
                <span className="mt-1 line-clamp-2 block text-[11px] leading-relaxed text-fg-subtle">
                  {excerpt(r.markdown)}
                </span>
                <span className="mt-2 flex items-center gap-1.5 text-[10.5px] text-fg-faint">
                  <Icon name="Clock" className="h-3 w-3" />
                  {dateFmt.format(new Date(r.createdAt))}
                </span>
                <span className="mt-1 flex items-center gap-1.5 text-[10.5px] text-fg-faint">
                  {r.pipelineId ? (
                    <>
                      <Icon name="Cable" className="h-3 w-3" />
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          openReportPipeline(r);
                        }}
                        title={t("reports.openPipeline")}
                        className="truncate text-left transition hover:text-info-fg"
                      >
                        {r.pipelineName}
                      </button>
                    </>
                  ) : (
                    <>
                      <Icon name="Bot" className="h-3 w-3" />
                      <span className="truncate">{t("reports.standaloneSource")}</span>
                    </>
                  )}
                </span>
                {r.tags.length > 0 && (
                  <span className="mt-1.5 flex flex-wrap gap-1">
                    {r.tags.map((tag) => {
                      const isActive = isTagSelected(tag);
                      return (
                        <button
                          key={tag}
                          aria-pressed={isActive}
                          title={isActive ? t("common.clear") : t("reports.filterByTag", { tag })}
                          onClick={(e) => {
                            e.stopPropagation();
                            toggleTag(tag);
                          }}
                          className={cn(
                            "flex items-center gap-1 rounded px-1.5 py-px font-mono text-[10px] transition",
                            isActive
                              ? "bg-info/20 text-info-fg hover:bg-info/30"
                              : "bg-ink-800 text-fg-subtle hover:bg-ink-700 hover:text-fg",
                          )}
                        >
                          {isActive && <Icon name="X" className="h-2.5 w-2.5" />}
                          #{tag}
                        </button>
                      );
                    })}
                  </span>
                )}
              </div>
            ))}
          </div>

          <article className="fade-in min-w-0 flex-1 overflow-y-auto px-7 py-6">
            {active ? (
              <div className="mx-auto max-w-[680px]">
                <div className="flex flex-wrap items-center gap-2 text-[11px] text-fg-faint">
                  <Icon name="Clock" className="h-3 w-3" />
                  {dateFmt.format(new Date(active.createdAt))}
                  <span className="h-3 w-px bg-ink-700" />
                  {active.pipelineId ? (
                    <>
                      <Icon name="Cable" className="h-3 w-3" />
                      <button
                        onClick={() => openReportPipeline(active)}
                        title={t("reports.openPipeline")}
                        className="truncate transition hover:text-info-fg"
                      >
                        {active.pipelineName}
                      </button>
                    </>
                  ) : (
                    <>
                      <Icon name="Bot" className="h-3 w-3" />
                      <span className="truncate">{t("reports.standaloneSource")}</span>
                    </>
                  )}
                  {active.tags.length > 0 && (
                    <>
                      <span className="h-3 w-px bg-ink-700" />
                      {active.tags.map((tag) => {
                        const isActive = isTagSelected(tag);
                        return (
                          <button
                            key={tag}
                            aria-pressed={isActive}
                            title={isActive ? t("common.clear") : t("reports.filterByTag", { tag })}
                            onClick={() => toggleTag(tag)}
                            className={cn(
                              "flex items-center gap-1 rounded px-1.5 py-px font-mono text-[10px] transition",
                              isActive
                                ? "bg-info/20 text-info-fg hover:bg-info/30"
                                : "bg-ink-800 text-fg-subtle hover:bg-ink-700 hover:text-fg",
                            )}
                          >
                            {isActive && <Icon name="X" className="h-2.5 w-2.5" />}
                            #{tag}
                          </button>
                        );
                      })}
                    </>
                  )}
                  <Button
                    variant="ghost"
                    icon="Copy"
                    className="ml-auto h-6"
                    onClick={() => navigator.clipboard?.writeText(active.markdown)}
                  >
                    {t("common.copy")}
                  </Button>
                </div>

                <h1 className="mt-3 text-[22px] font-semibold tracking-tight text-fg">{active.title}</h1>

                <div className="my-5 h-px bg-ink-750" />

                {/* report content is user-generated data — rendered verbatim as markdown */}
                <MarkdownRenderer text={active.markdown} />
              </div>
            ) : null}
          </article>
        </div>
      )}
      </div>
    </ViewShell>
  );
}
