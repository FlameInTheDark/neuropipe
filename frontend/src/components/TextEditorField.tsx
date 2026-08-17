import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Eye, FileText, Maximize2, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { MarkdownContent } from "@/components/MarkdownContent";

/**
 * TextEditorExpandButton is the small button rendered on the inspector label
 * line. It opens the full-screen text editor modal.
 */
export function TextEditorExpandButton({
  value,
  onChange,
  placeholder,
  multiline,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  multiline?: boolean;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="ml-auto h-5 px-1.5 text-[10px] text-zinc-500 hover:text-zinc-200"
        onClick={() => setOpen(true)}
        aria-label={t("textEditor.expand", "Expand editor")}
      >
        <Maximize2 className="size-3" />
      </Button>
      {open ? (
        <TextEditorModal
          value={value}
          onChange={onChange}
          placeholder={placeholder}
          onClose={() => setOpen(false)}
        />
      ) : null}
    </>
  );
}

/**
 * TextEditorField renders the inline input/textarea without a label (the label
 * is rendered by the parent ConfigFieldRow). The expand button is injected
 * into the label line via TextEditorExpandButton.
 */
export function TextEditorField({
  value,
  onChange,
  placeholder,
  multiline,
  ariaLabel,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  multiline?: boolean;
  ariaLabel?: string;
}) {
  return (
    <div className="block">
      {multiline ? (
        <textarea
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className="min-h-48 w-full resize-y rounded-md border border-zinc-700 bg-zinc-950 px-2.5 py-2 font-mono text-xs leading-5 text-zinc-200 outline-none focus:border-zinc-500"
          placeholder={placeholder}
          aria-label={ariaLabel}
        />
      ) : (
        <input
          type="text"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className="h-8 w-full rounded-md border border-zinc-700 bg-zinc-950 px-2.5 text-sm text-zinc-100 outline-none placeholder:text-zinc-600 focus:border-zinc-500"
          placeholder={placeholder}
          aria-label={ariaLabel}
        />
      )}
    </div>
  );
}

function TextEditorModal({
  value,
  onChange,
  placeholder,
  onClose,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<"editor" | "markdown">("editor");
  const [draft, setDraft] = useState(value);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onChange(draft);
        onClose();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [draft, onChange, onClose]);

  const handleSave = () => {
    onChange(draft);
    onClose();
  };

  return createPortal(
    <div
      className="fixed inset-0 z-[110] flex items-center justify-center bg-black/70 p-5 backdrop-blur-[2px]"
      onPointerDown={(event) => {
        if (event.target === event.currentTarget) { onChange(draft); onClose(); }
      }}
    >
      <section className="flex h-[calc(100vh-80px)] w-full max-w-5xl flex-col overflow-hidden rounded-xl border border-zinc-700 bg-zinc-950 shadow-2xl shadow-black/70">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-zinc-800 px-5 py-3">
          <div className="flex items-center gap-1 rounded-md border border-zinc-800 bg-zinc-900 p-0.5">
            <button
              type="button"
              className={`flex items-center gap-1.5 rounded px-2.5 py-1 text-xs font-medium transition-colors ${mode === "editor" ? "bg-zinc-700 text-zinc-100" : "text-zinc-500 hover:text-zinc-300"}`}
              onClick={() => setMode("editor")}
            >
              <FileText className="size-3.5" />
              {t("textEditor.editor", "Editor")}
            </button>
            <button
              type="button"
              className={`flex items-center gap-1.5 rounded px-2.5 py-1 text-xs font-medium transition-colors ${mode === "markdown" ? "bg-zinc-700 text-zinc-100" : "text-zinc-500 hover:text-zinc-300"}`}
              onClick={() => setMode("markdown")}
            >
              <Eye className="size-3.5" />
              {t("textEditor.markdown", "Markdown")}
            </button>
          </div>
          <Button variant="ghost" size="sm" className="size-7 p-0" onClick={() => { onChange(draft); onClose(); }} aria-label="Close">
            <X className="size-4" />
          </Button>
        </div>

        {/* Body */}
        <div className="min-h-0 flex-1 overflow-hidden">
          {mode === "editor" ? (
            <textarea
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              className="h-full w-full resize-none bg-zinc-950 p-5 font-mono text-sm leading-6 text-zinc-200 outline-none"
              placeholder={placeholder}
              autoFocus
            />
          ) : (
            <div className="muted-scroll h-full overflow-y-auto p-5">
              {draft.trim() ? (
                <MarkdownContent markdown={draft} />
              ) : (
                <p className="text-sm text-zinc-600">{placeholder || t("textEditor.emptyPreview", "Nothing to preview yet.")}</p>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-2 border-t border-zinc-800 px-5 py-3">
          <Button variant="ghost" onClick={() => { onChange(value); onClose(); }}>
            {t("common.cancel", "Cancel")}
          </Button>
          <Button onClick={handleSave}>
            {t("common.save", "Save")}
          </Button>
        </div>
      </section>
    </div>,
    document.body,
  );
}
