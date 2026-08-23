import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import type { Report } from "@/lib/types";
import type { Workspace } from "@/features/workspace/useWorkspace";
import type { NavApi } from "@/features/workspace/useWorkspaceNav";
import { ask } from "@/stores/confirmation";
import { SearchInput, ViewShell, EmptyState } from "../components/ViewShell";
import { Button } from "../components/ui";
import { Icon } from "../components/icons";
import { Dropdown } from "../components/Dropdown";
import { useCtxMenu } from "../components/ContextMenu";
import { cn } from "../utils/cn";

const SORTS = [
  { value: "newest", label: "reports.newest", icon: "Clock" },
  { value: "oldest", label: "reports.oldest", icon: "History" },
];

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
  const ctx = useCtxMenu();

  const filtered = useMemo(() => {
    const query = q.toLowerCase();
    const hits = reports.filter(
      (r) =>
        r.title.toLowerCase().includes(query) || r.pipelineName.toLowerCase().includes(query),
    );
    return sort === "newest"
      ? [...hits].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
      : [...hits].sort((a, b) => Date.parse(a.createdAt) - Date.parse(b.createdAt));
  }, [reports, q, sort]);

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

  return (
    <ViewShell
      title={t("reports.title")}
      subtitle={t("status.count", { count: reports.length })}
      padded={false}
    >
      <div className="border-b border-seam px-4 py-2">
        <div className="flex items-center gap-2">
          <SearchInput
            value={q}
            onChange={setQ}
            placeholder={t("reports.search")}
            className="w-[280px]"
          />
          <Dropdown
            value={sort}
            onChange={setSort}
            className="w-[160px]"
            options={SORTS.map((s) => ({ ...s, label: t(s.label) }))}
          />
          <span className="ml-auto text-[11.5px] text-ink-500">{t("reports.description")}</span>
        </div>
      </div>

      {filtered.length === 0 ? (
        <EmptyState icon="FileText" title={t("reports.emptyTitle")} hint={t("reports.emptyDescription")} />
      ) : (
        <div className="flex h-full min-h-0">
          <div className="w-[300px] shrink-0 overflow-y-auto border-r border-seam p-2.5">
            {filtered.map((r) => (
              <button
                key={r.id}
                onClick={() => setSelectedId(r.id)}
                onContextMenu={(e) => reportMenu(e, r)}
                className={cn(
                  "mb-2 block w-full rounded-xl border p-3 text-left transition",
                  active?.id === r.id
                    ? "border-ink-500 bg-ink-800/70"
                    : "border-ink-700/80 bg-ink-850/50 hover:border-ink-600 hover:bg-ink-850",
                )}
              >
                <span className="block truncate text-[12.5px] font-semibold text-ink-50">{r.title}</span>
                <span className="mt-1 line-clamp-2 block text-[11px] leading-relaxed text-ink-400">
                  {excerpt(r.markdown)}
                </span>
                <span className="mt-2 flex items-center gap-1.5 text-[10.5px] text-ink-500">
                  <Icon name="Clock" className="h-3 w-3" />
                  {dateFmt.format(new Date(r.createdAt))}
                </span>
                <span className="mt-1 flex items-center gap-1.5 text-[10.5px] text-ink-500">
                  <Icon name="Cable" className="h-3 w-3" />
                  {r.pipelineName}
                </span>
              </button>
            ))}
          </div>

          <article className="fade-in min-w-0 flex-1 overflow-y-auto px-7 py-6">
            {active ? (
              <div className="mx-auto max-w-[680px]">
                <div className="flex flex-wrap items-center gap-2 text-[11px] text-ink-500">
                  <Icon name="Clock" className="h-3 w-3" />
                  {dateFmt.format(new Date(active.createdAt))}
                  <span className="h-3 w-px bg-ink-700" />
                  <Icon name="Cable" className="h-3 w-3" />
                  {active.pipelineName}
                  {active.tags.length > 0 && (
                    <>
                      <span className="h-3 w-px bg-ink-700" />
                      {active.tags.map((tag) => (
                        <span key={tag} className="rounded bg-ink-800 px-1.5 py-px font-mono text-[10px] text-ink-300">
                          #{tag}
                        </span>
                      ))}
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

                <h1 className="mt-3 text-[22px] font-semibold tracking-tight text-ink-50">{active.title}</h1>

                <div className="my-5 h-px bg-ink-750" />

                {/* report content is user-generated data — rendered verbatim as markdown */}
                <MarkdownBody markdown={active.markdown} />
              </div>
            ) : null}
          </article>
        </div>
      )}
    </ViewShell>
  );
}

/** Local markdown renderer for user-authored report content. */
function MarkdownBody({ markdown }: { markdown: string }) {
  return (
    <div className="space-y-3 text-[13.5px] leading-relaxed text-ink-200 [&_a]:text-sky-300 [&_blockquote]:border-l-2 [&_blockquote]:border-ink-600 [&_blockquote]:pl-3 [&_code]:rounded [&_code]:bg-ink-800 [&_code]:px-1 [&_code]:font-mono [&_code]:text-[12px] [&_h1]:text-[18px] [&_h1]:font-semibold [&_h1]:text-ink-50 [&_h2]:mt-4 [&_h2]:text-[15px] [&_h2]:font-semibold [&_h2]:text-ink-50 [&_h3]:text-[13.5px] [&_h3]:font-semibold [&_h3]:text-ink-100 [&_li]:ml-4 [&_li]:list-disc [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:border [&_pre]:border-ink-700 [&_pre]:bg-ink-950/60 [&_pre]:p-3 [&_table]:w-full [&_td]:border-t [&_td]:border-seam [&_td]:px-2 [&_td]:py-1 [&_th]:px-2 [&_th]:py-1 [&_th]:text-left">
      <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeHighlight]}>
        {markdown}
      </ReactMarkdown>
    </div>
  );
}
