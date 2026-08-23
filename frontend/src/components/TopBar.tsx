import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Application, Window } from "@wailsio/runtime";
import { Badge, Button, Divider, IconButton } from "./ui";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";

function EditableName({
  name,
  label,
  onRename,
}: {
  name: string;
  label: string;
  onRename?: (v: string) => void;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(name);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => setDraft(name), [name]);

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [editing]);

  const commit = () => {
    setEditing(false);
    const next = draft.trim();
    if (next && next !== name) onRename?.(next);
    else setDraft(name);
  };

  if (editing) {
    return (
      <input
        ref={inputRef}
        value={draft}
        aria-label={label}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          e.stopPropagation();
          if (e.key === "Enter") commit();
          if (e.key === "Escape") {
            setDraft(name);
            setEditing(false);
          }
        }}
        style={{ width: `${Math.max(90, Math.min(320, draft.length * 7.4 + 22))}px` }}
        className="h-[26px] rounded-md border border-ink-500 bg-ink-850 px-1.5 text-[13px] font-medium text-ink-50 outline-none"
      />
    );
  }

  return (
    <Tooltip content={t("editor.rename")} side="bottom" delay={450}>
      <button
        onClick={() => setEditing(true)}
        className="group flex min-w-0 items-center gap-1.5 rounded-md px-1.5 py-[3px] transition hover:bg-ink-800"
      >
        <span className="truncate text-[13px] font-medium text-ink-50">{name}</span>
        <Icon
          name="Pencil"
          className="h-3 w-3 shrink-0 text-ink-600 opacity-0 transition group-hover:opacity-100"
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
  busy?: string | null;
  onRename?: (name: string) => void;
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
      className="flex h-11 shrink-0 items-center gap-2 border-b border-seam bg-ink-950 pr-2 pl-3 select-none"
    >
      {/* breadcrumb */}
      <nav style={noDragStyle} className="flex min-w-0 items-center gap-1.5 text-[13px]">
        {inEditor ? (
          <>
            <Tooltip content={parentTitle} side="bottom">
              <button
                onClick={onBack}
                aria-label={parentTitle}
                className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-ink-300 transition hover:bg-ink-800 hover:text-ink-50"
              >
                <Icon name="ArrowLeft" className="h-4 w-4" />
              </button>
            </Tooltip>
            <button
              onClick={onBack}
              className="rounded px-1 py-0.5 text-ink-400 transition hover:bg-ink-800 hover:text-ink-100"
            >
              {parentTitle}
            </button>
            <Icon name="ChevronRight" className="h-3 w-3 shrink-0 text-ink-600" />
            <EditableName name={pipelineName ?? ""} label={t("editor.rename")} onRename={onRename} />
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
              <span className="flex shrink-0 items-center gap-1.5 pl-1 text-[11.5px] text-ink-400">
                <span className="h-1.5 w-1.5 rounded-full bg-amber-400/80" />
                {t("common.unsaved")}
              </span>
            )}
          </>
        ) : (
          <span className="truncate font-medium text-ink-50">{viewTitle}</span>
        )}
      </nav>

      {/* command bar */}
      <button
        onClick={onCommand}
        style={noDragStyle}
        className="mx-auto hidden h-7 w-[300px] items-center gap-2 rounded-md border border-ink-700/70 bg-ink-900/80 px-2.5 text-[12px] text-ink-400 transition hover:border-ink-600 hover:text-ink-200 lg:flex"
      >
        <Icon name="Search" className="h-3.5 w-3.5" />
        <span>{t("palette.search")}</span>
        <kbd className="ml-auto rounded border border-ink-700 bg-ink-850 px-1 font-mono text-[10px] text-ink-500">
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
            className="hover:bg-rose-500/80 hover:text-white"
            onClick={() => void Application.Quit().catch(() => undefined)}
          />
        </div>
      </div>
    </header>
  );
}


