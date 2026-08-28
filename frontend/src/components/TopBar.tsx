import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Application, Window } from "@wailsio/runtime";
import { Badge, Button, Divider, IconButton } from "./ui";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";

function EditableName({
  name,
  label,
  description,
  descriptionLabel,
  onRename,
}: {
  name: string;
  label: string;
  description?: string;
  descriptionLabel?: string;
  /** Receives the committed name plus trimmed description (may be ""). */
  onRename?: (name: string, description?: string) => void;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(name);
  const [draftDescription, setDraftDescription] = useState(description ?? "");
  const inputRef = useRef<HTMLInputElement>(null);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => setDraft(name), [name]);
  useEffect(() => setDraftDescription(description ?? ""), [description]);

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [editing]);

  /* close the details popover when clicking anywhere else */
  useEffect(() => {
    if (!editing) return;
    const onDown = (e: PointerEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) commitRef.current();
    };
    window.addEventListener("pointerdown", onDown);
    return () => window.removeEventListener("pointerdown", onDown);
  }, [editing]);

  const commitRef = useRef(() => {});
  commitRef.current = () => {
    setEditing(false);
    const next = draft.trim();
    const nextDescription = draftDescription.trim();
    if ((!next || next === name) && nextDescription === (description ?? "")) {
      setDraft(name);
      setDraftDescription(description ?? "");
      return;
    }
    onRename?.(next || name, nextDescription);
  };

  const commit = () => commitRef.current();

  if (editing) {
    return (
      <div ref={wrapRef} className="relative">
        <input
          ref={inputRef}
          value={draft}
          aria-label={label}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            e.stopPropagation();
            if (e.key === "Enter") commit();
            if (e.key === "Escape") {
              setDraft(name);
              setDraftDescription(description ?? "");
              setEditing(false);
            }
          }}
          style={{ width: `${Math.max(90, Math.min(320, draft.length * 7.4 + 22))}px` }}
          className="h-[26px] rounded-md border border-ink-500 bg-ink-850 px-1.5 text-[13px] font-medium text-fg outline-none"
        />
        <div className="absolute top-[calc(100%+6px)] left-0 z-50 w-[340px] rounded-lg border border-ink-650 bg-ink-900 p-3 shadow-[0_24px_60px_-16px_rgba(0,0,0,0.9)]">
          <label className="block">
            <span className="mb-1 block text-[11px] font-medium text-fg-subtle">{label}</span>
            <input
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                e.stopPropagation();
                if (e.key === "Enter") commit();
                if (e.key === "Escape") {
                  setDraft(name);
                  setDraftDescription(description ?? "");
                  setEditing(false);
                }
              }}
              className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2 text-[12.5px] text-fg outline-none focus:border-ink-500"
            />
          </label>
          <label className="mt-2.5 block">
            <span className="mb-1 block text-[11px] font-medium text-fg-subtle">
              {descriptionLabel ?? t("editor.rename")}
            </span>
            <textarea
              rows={3}
              value={draftDescription}
              onChange={(e) => setDraftDescription(e.target.value)}
              onKeyDown={(e) => {
                e.stopPropagation();
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  commit();
                }
                if (e.key === "Escape") {
                  setDraft(name);
                  setDraftDescription(description ?? "");
                  setEditing(false);
                }
              }}
              className="w-full resize-none rounded-md border border-ink-700 bg-ink-850 px-2 py-1.5 text-[12.5px] text-fg outline-none focus:border-ink-500"
            />
          </label>
          <div className="mt-2 flex items-center justify-end gap-2">
            <button
              type="button"
              onClick={() => {
                setDraft(name);
                setDraftDescription(description ?? "");
                setEditing(false);
              }}
              className="h-7 rounded-md border border-ink-700 bg-ink-850 px-3 text-[11.5px] text-fg-muted transition hover:bg-ink-750"
            >
              {t("common.cancel")}
            </button>
            <button
              type="button"
              onClick={commit}
              className="h-7 rounded-md bg-ink-50 px-3 text-[11.5px] font-medium text-fg-onEmphasis transition hover:bg-ink-25"
            >
              {t("common.save")}
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <Tooltip content={t("editor.rename")} side="bottom" delay={450}>
      <button
        onClick={() => setEditing(true)}
        className="group flex min-w-0 items-center gap-1.5 rounded-md px-1.5 py-[3px] transition hover:bg-ink-800"
      >
        <span className="truncate text-[13px] font-medium text-fg">{name}</span>
        <Icon
          name="Pencil"
          className="h-3 w-3 shrink-0 text-fg-faint opacity-0 transition group-hover:opacity-100"
        />
      </button>
    </Tooltip>
  );
}

export function TopBar({
  inEditor,
  viewTitle,
  parentTitle = "Pipelines",
  pipelineName,
  version,
  executorName,
  description,
  descriptionLabel,
  busy,
  onRename,
  dirty,
  running,
  onBack,
  onSave,
  onRun,
  onStop,
  onPublish,
  onCommand,
  leftOpen,
  rightOpen,
  toggleLeft,
  toggleRight,
}: {
  inEditor: boolean;
  viewTitle: string;
  parentTitle?: string;
  pipelineName?: string;
  version?: string;
  /** Set when the edited pipeline targets a remote executor. */
  executorName?: string;
  /** Current description shown/edited alongside the name. */
  description?: string;
  descriptionLabel?: string;
  busy?: string | null;
  onRename?: (name: string, description?: string) => void;
  dirty: boolean;
  running: boolean;
  onBack: () => void;
  onSave: () => void;
  onRun: () => void;
  onStop: () => void;
  onPublish: () => void;
  onCommand: () => void;
  leftOpen: boolean;
  rightOpen: boolean;
  toggleLeft: () => void;
  toggleRight: () => void;
}) {
  const { t } = useTranslation();
  const [maximised, setMaximised] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const sync = async () => {
      try {
        const value = await Window.IsMaximised();
        if (!cancelled) setMaximised(Boolean(value));
      } catch {
        /* outside Wails */
      }
    };
    void sync();
    return () => {
      cancelled = true;
    };
  }, []);

  const toggleMaximise = async () => {
    try {
      await Window.ToggleMaximise();
      window.setTimeout(() => {
        void Window.IsMaximised().then((v) => setMaximised(Boolean(v))).catch(() => undefined);
      }, 80);
    } catch {
      /* outside Wails */
    }
  };

  /* Frameless window: the bar itself is the OS drag region (same mechanism as
     the previous UI). Interactive clusters below opt out with no-drag so
     buttons, menus, and the inline rename field keep working. */
  const dragStyle = { "--wails-draggable": "drag" } as React.CSSProperties;
  const noDragStyle = { "--wails-draggable": "no-drag" } as React.CSSProperties;

  return (
    <header
      style={dragStyle}
      onDoubleClick={(e) => {
        // only bare titlebar area toggles maximize — not buttons/inputs within
        if (e.target === e.currentTarget) void toggleMaximise();
      }}
      className="relative flex h-11 shrink-0 items-center gap-2 border-b border-seam bg-ink-950 pr-2 pl-3 select-none"
    >
      {/* breadcrumb */}
      <nav style={noDragStyle} className="flex min-w-0 items-center gap-1.5 text-[13px]">
        {inEditor ? (
          <>
            <Tooltip content={parentTitle} side="bottom">
              <button
                onClick={onBack}
                aria-label={parentTitle}
                className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-fg-subtle transition hover:bg-ink-800 hover:text-fg"
              >
                <Icon name="ArrowLeft" className="h-4 w-4" />
              </button>
            </Tooltip>
            <button
              onClick={onBack}
              className="rounded px-1 py-0.5 text-fg-subtle transition hover:bg-ink-800 hover:text-fg"
            >
              {parentTitle}
            </button>
            <Icon name="ChevronRight" className="h-3 w-3 shrink-0 text-fg-faint" />
            <EditableName
              name={pipelineName ?? ""}
              label={t("editor.rename")}
              description={description}
              descriptionLabel={descriptionLabel}
              onRename={(n, d) => onRename?.(n, d)}
            />
            {executorName && (
              <Badge tone="muted" className="ml-0.5 inline-flex items-center gap-1 border-violet-500/30 bg-violet-500/10 text-violet-300">
                {executorName}
              </Badge>
            )}
            {version && (
              <Badge tone="muted" className="ml-0.5">
                {version}
              </Badge>
            )}
            {dirty && (
              <span className="flex shrink-0 items-center gap-1.5 pl-1 text-[11.5px] text-fg-subtle">
                <span className="h-1.5 w-1.5 rounded-full bg-warning/80" />
                {t("common.unsaved")}
              </span>
            )}
          </>
        ) : (
          <span className="truncate font-medium text-fg">{viewTitle}</span>
        )}
      </nav>

      {/* command bar — always centered relative to the title bar, regardless of
          the widths of the breadcrumb (left) and actions (right). */}
      <button
        onClick={onCommand}
        style={noDragStyle}
        className="pointer-events-auto absolute left-1/2 top-1/2 hidden h-7 w-[300px] -translate-x-1/2 -translate-y-1/2 items-center gap-2 rounded-md border border-ink-700/70 bg-ink-900/80 px-2.5 text-[12px] text-fg-subtle transition hover:border-ink-600 hover:text-fg-muted lg:flex"
      >
        <Icon name="Search" className="h-3.5 w-3.5" />
        <span>{t("palette.search")}</span>
        <kbd className="ml-auto rounded border border-ink-700 bg-ink-850 px-1 font-mono text-[10px] text-fg-faint">
          ⌘K
        </kbd>
      </button>

      {/* actions */}
      <div style={noDragStyle} className="ml-auto flex items-center gap-1">
        {inEditor && (
          <>
            <IconButton icon="PanelLeft" label={t("editor.library")} active={leftOpen} onClick={toggleLeft} />
            <IconButton icon="PanelRight" label={t("editor.inspector")} active={rightOpen} onClick={toggleRight} />
            <Divider className="mx-1" />
            <Button icon="Save" onClick={onSave} disabled={busy === "save"}>
              {busy === "save" ? t("common.saving") : t("common.save")}
            </Button>
            {running ? (
              <Button icon="Square" variant="solid" onClick={onStop}>
                {t("triggers.stop")}
              </Button>
            ) : (
              <Button icon="Play" variant="solid" onClick={onRun}>
                {t("editor.runDraft")}
              </Button>
            )}
            <Button icon="UploadCloud" variant="primary" onClick={onPublish} disabled={busy === "publish"}>
              {busy === "publish" ? t("common.saving") : t("editor.publish")}
            </Button>
            <Divider className="mx-1" />
          </>
        )}
        <div className="flex items-center gap-0.5">
          <IconButton
            icon="Minus"
            label={t("titlebar.minimise")}
            size="sm"
            onClick={() => void Window.Minimise().catch(() => undefined)}
          />
          <IconButton
            icon={maximised ? "Expand" : "Frame"}
            label={maximised ? t("titlebar.restore") : t("titlebar.maximise")}
            size="sm"
            onClick={() => void toggleMaximise()}
          />
          <IconButton
            icon="X"
            label={t("titlebar.close")}
            size="sm"
            className="hover:bg-danger hover:text-white"
            onClick={() => void Application.Quit().catch(() => undefined)}
          />
        </div>
      </div>
    </header>
  );
}


