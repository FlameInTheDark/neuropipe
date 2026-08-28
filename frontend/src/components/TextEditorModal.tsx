import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { MarkdownRenderer } from "./MarkdownRenderer";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";
import { useTranslation } from "react-i18next";
import { cn } from "../utils/cn";

type Tab = "edit" | "preview" | "split";

export function TextEditorModal({
  title,
  value,
  placeholder,
  onSave,
  onClose,
}: {
  title: string;
  value: string;
  placeholder?: string;
  onSave: (v: string) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(value);
  const [tab, setTab] = useState<Tab>("edit");
  const [wordWrap, setWordWrap] = useState(true);
  const [fontSize, setFontSize] = useState(13);
  const textRef = useRef<HTMLTextAreaElement>(null);
  const dirty = draft !== value;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        onSave(draft);
      }
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "b") {
        if (document.activeElement === textRef.current) {
          e.preventDefault();
          insert("**", "**");
        }
      }
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "i") {
        if (document.activeElement === textRef.current) {
          e.preventDefault();
          insert("_", "_");
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draft, onClose, onSave]);

  useEffect(() => {
    if (tab !== "preview") textRef.current?.focus();
  }, [tab]);

  const stats = useMemo(() => {
    const chars = draft.length;
    const words = draft.trim() ? draft.trim().split(/\s+/).length : 0;
    const lines = draft.split("\n").length;
    // ~200 wpm average reading speed
    const readMin = Math.max(1, Math.round(words / 200));
    return { chars, words, lines, readMin };
  }, [draft]);

  const insert = (before: string, after = "") => {
    const el = textRef.current;
    if (!el) return;
    const s = el.selectionStart;
    const e = el.selectionEnd;
    const sel = draft.slice(s, e);
    const next = draft.slice(0, s) + before + sel + after + draft.slice(e);
    setDraft(next);
    requestAnimationFrame(() => {
      el.focus();
      const cursor = sel.length ? s + before.length + sel.length + after.length : s + before.length;
      el.setSelectionRange(cursor, cursor);
    });
  };

  return createPortal(
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center bg-black/70 backdrop-blur-[3px] p-4 sm:p-6"
      onClick={onClose}
    >
      <div
        className={cn(
          "pop-in flex w-full flex-col overflow-hidden rounded-xl border border-ink-650 bg-ink-900 shadow-[0_40px_100px_-30px_rgba(0,0,0,0.95)]",
          // grow with viewport, cap at 1440×1000, min ~640/500
          "h-[min(96vh,1000px)] max-w-[min(96vw,1440px)]",
        )}
        onClick={(e) => e.stopPropagation()}
      >
        {/* ── header ── */}
        <div className="flex h-11 shrink-0 items-center gap-2.5 border-b border-seam px-4">
          <Icon name="FileText" className="h-4 w-4 text-fg-subtle" />
          <h2 className="truncate text-[13px] font-semibold text-fg">{title}</h2>
          {dirty && (
            <span className="flex items-center gap-1.5 text-[11px] text-fg-subtle">
              <span className="h-1.5 w-1.5 rounded-full bg-warning/80" />
              Modified
            </span>
          )}

          <div className="ml-auto flex items-center gap-1">
            <div className="flex items-center gap-0.5 rounded-md border border-ink-700 bg-ink-850 p-0.5">
              {(["edit", "split", "preview"] as const).map((tb) => (
                <button
                  key={tb}
                  onClick={() => setTab(tb)}
                  className={cn(
                    "flex h-[22px] items-center gap-1.5 rounded px-2 text-[11px] capitalize transition",
                    tab === tb ? "bg-ink-700 text-fg" : "text-fg-subtle hover:text-fg",
                  )}
                >
                  <Icon
                    name={tb === "edit" ? "Pencil" : tb === "split" ? "PanelRight" : "Eye"}
                    className="h-3 w-3"
                  />
                  {t(`textEditor.tab_${tb}`)}
                </button>
              ))}
            </div>

            <span className="mx-1 h-4 w-px bg-ink-700" />

            <Tooltip content={t("common.close")} hint="Esc" side="bottom">
              <button
                onClick={onClose}
                className="grid h-7 w-7 place-items-center rounded-md text-fg-subtle transition hover:bg-ink-800 hover:text-fg"
              >
                <Icon name="X" className="h-4 w-4" />
              </button>
            </Tooltip>
          </div>
        </div>

        {/* ── toolbar (only when edit surface visible) ── */}
        {tab !== "preview" && (
          <div className="flex h-9 shrink-0 items-center gap-0.5 border-b border-seam px-3">
            <TBtn icon="Bold" label={t("textEditor.bold")} hint="⌘B" onClick={() => insert("**", "**")} />
            <TBtn icon="Italic" label={t("textEditor.italic")} hint="⌘I" onClick={() => insert("_", "_")} />
            <TBtn icon="Code" label={t("textEditor.inlineCode")} onClick={() => insert("`", "`")} />
            <TBtn icon="Minus" label={t("textEditor.rule")} onClick={() => insert("\n\n---\n\n")} />

            <span className="mx-1 h-4 w-px bg-ink-700" />

            <TBtn icon="List" label={t("textEditor.bullets")} onClick={() => insert("\n- ")} />
            <TBtn icon="ListFilter" label={t("textEditor.numbered")} onClick={() => insert("\n1. ")} />
            <TBtn icon="Braces" label={t("textEditor.codeBlock")} onClick={() => insert("\n```\n", "\n```\n")} />
            <TBtn icon="Table2" label={t("textEditor.table")} onClick={() => insert("\n\n| Column | Column |\n|--------|--------|\n| Cell   | Cell   |\n\n")} />
            <TBtn icon="ExternalLink" label={t("textEditor.link")} onClick={() => insert("[", "](https://)")} />
            <TBtn icon="AlertTriangle" label={t("textEditor.quote")} onClick={() => insert("\n> ")} />

            <span className="mx-1 h-4 w-px bg-ink-700" />

            <Tooltip content={wordWrap ? t("textEditor.wrapOff") : t("textEditor.wrapOn")} side="bottom">
              <button
                onClick={() => setWordWrap(!wordWrap)}
                className={cn(
                  "grid h-6 w-6 place-items-center rounded-md transition",
                  wordWrap ? "bg-ink-750 text-fg" : "text-fg-subtle hover:text-fg",
                )}
              >
                <Icon name="Maximize2" className="h-3.5 w-3.5" />
              </button>
            </Tooltip>

            <div className="ml-1 flex items-center gap-0.5">
              <button
                onClick={() => setFontSize((s) => Math.max(10, s - 1))}
                className="grid h-6 w-6 place-items-center rounded text-fg-subtle hover:text-fg"
              >
                <Icon name="Minus" className="h-3 w-3" />
              </button>
              <span className="w-6 text-center font-mono text-[10px] text-fg-subtle">{fontSize}</span>
              <button
                onClick={() => setFontSize((s) => Math.min(20, s + 1))}
                className="grid h-6 w-6 place-items-center rounded text-fg-subtle hover:text-fg"
              >
                <Icon name="Plus" className="h-3 w-3" />
              </button>
            </div>
          </div>
        )}

        {/* ── body ── */}
        <div className="min-h-0 flex-1 overflow-hidden">
          {tab === "edit" && (
            <Editor
              ref={textRef}
              value={draft}
              onChange={setDraft}
              placeholder={placeholder}
              fontSize={fontSize}
              wordWrap={wordWrap}
            />
          )}

          {tab === "preview" && (
            <div className="h-full overflow-y-auto px-8 py-6">
              <MarkdownRenderer text={draft} />
            </div>
          )}

          {tab === "split" && (
            <div className="grid h-full grid-cols-2 divide-x divide-seam">
              <Editor
                ref={textRef}
                value={draft}
                onChange={setDraft}
                placeholder={placeholder}
                fontSize={fontSize}
                wordWrap={wordWrap}
              />
              <div className="h-full overflow-y-auto px-6 py-5">
                <MarkdownRenderer text={draft} />
              </div>
            </div>
          )}
        </div>

        {/* ── footer ── */}
        <div className="flex h-10 shrink-0 items-center gap-3 border-t border-seam px-4 text-[11px] text-fg-faint">
          <span>{stats.lines.toLocaleString()} lines</span>
          <span className="h-3 w-px bg-ink-700" />
          <span>{t("textEditor.words", { count: stats.words.toLocaleString() })}</span>
          <span className="h-3 w-px bg-ink-700" />
          <span>{t("textEditor.chars", { count: stats.chars.toLocaleString() })}</span>
          <span className="h-3 w-px bg-ink-700" />
          <span>{t("textEditor.readTime", { count: stats.readMin })}</span>

          <div className="ml-auto flex items-center gap-2">
            <kbd className="rounded border border-ink-700 bg-ink-850 px-1 py-px font-mono text-[10px] text-fg-faint">⌘S</kbd>
            <kbd className="rounded border border-ink-700 bg-ink-850 px-1 py-px font-mono text-[10px] text-fg-faint">{t("textEditor.escClose")}</kbd>

            <button
              onClick={onClose}
              className="h-7 rounded-md border border-ink-700 bg-ink-850 px-3 text-[12px] text-fg-muted transition hover:bg-ink-750"
            >
              {t("common.cancel")}
            </button>
            <button
              onClick={() => onSave(draft)}
              disabled={!dirty}
              className={cn(
                "h-7 rounded-md px-3 text-[12px] font-medium transition",
                dirty
                  ? "bg-ink-50 text-fg-onEmphasis hover:bg-ink-25"
                  : "cursor-not-allowed bg-ink-800 text-fg-faint",
              )}
            >
              {t(`javascript.save`)}
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}

/* ── editor textarea ── */
const Editor = ({
  ref,
  value,
  onChange,
  placeholder,
  fontSize,
  wordWrap,
}: {
  ref: React.RefObject<HTMLTextAreaElement | null>;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  fontSize: number;
  wordWrap: boolean;
}) => (
  <textarea
    ref={ref}
    value={value}
    onChange={(e) => onChange(e.target.value)}
    placeholder={placeholder}
    spellCheck={false}
    style={{ fontSize, tabSize: 2 }}
    className={cn(
      "h-full w-full resize-none bg-transparent p-5 font-mono leading-[1.65] text-fg placeholder:text-fg-faint focus:outline-none",
      wordWrap ? "whitespace-pre-wrap break-words" : "whitespace-pre overflow-x-auto",
    )}
  />
);

/* ── toolbar button ── */
function TBtn({
  icon,
  label,
  hint,
  onClick,
}: {
  icon: string;
  label: string;
  hint?: string;
  onClick: () => void;
}) {
  return (
    <Tooltip content={label} hint={hint} side="bottom" delay={200}>
      <button
        onClick={onClick}
        className="grid h-6 w-6 place-items-center rounded-md text-fg-subtle transition hover:bg-ink-750 hover:text-fg active:scale-95"
      >
        <Icon name={icon} className="h-3.5 w-3.5" />
      </button>
    </Tooltip>
  );
}

/* ── markdown preview (react-markdown + gfm + hljs) ── */





