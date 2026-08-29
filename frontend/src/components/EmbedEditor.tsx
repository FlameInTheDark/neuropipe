import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Badge, Button, IconButton } from "./ui";
import { Modal } from "./primitives/Modal";
import {
  type EmbedDoc,
  type EmbedFieldSpec,
  type EmbedPin,
  type EmbedSpec,
  createEmbed,
  createField,
  embedsFromRawJSON,
  embedsToRawJSON,
  nextEmbedId,
  nextFieldId,
  normalizeEmbedDoc,
  serializeEmbedDoc,
} from "@/lib/embed";
import { EmbedForm } from "./embed-editor/EmbedForm";
import { DiscordPreview } from "./embed-editor/DiscordPreview";

/* ------------------------------------------------------------------ */
/* inspector entry point                                               */
/* ------------------------------------------------------------------ */

export function EmbedEditor({
  value,
  onChange,
  previewContent,
  previewAttachments,
}: {
  value: unknown;
  onChange: (value: unknown) => void;
  /** Message text from the node's own message field, for the live preview. */
  previewContent?: string;
  /** Attachment names from the node's file pins, for the live preview. */
  previewAttachments?: Array<{ name: string; size?: string }>;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const doc = useMemo(() => normalizeEmbedDoc(value), [value]);

  return (
    <div className="flex items-center justify-between gap-2 rounded-md border border-ink-700/70 bg-ink-850 px-2.5 py-[7px]">
      <span className="flex min-w-0 flex-col">
        <span className="text-[12.5px] font-medium text-fg">{t("embedEditor.openEditor")}</span>
        <span className="truncate text-[11px] text-fg-faint">
          {doc.embeds.length} {t("embedEditor.embedCount")} · {doc.pins.length} {t("embedEditor.variableCount")}
        </span>
      </span>
      <Button variant="solid" icon="Braces" onClick={() => setOpen(true)}>
        {t("embedEditor.openEditorButton")}
      </Button>
      {open ? (
        <EmbedEditorModal
          value={value}
          onChange={onChange}
          previewContent={previewContent ?? ""}
          previewAttachments={previewAttachments ?? []}
          onClose={() => setOpen(false)}
        />
      ) : null}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* modal                                                               */
/* ------------------------------------------------------------------ */

const HISTORY_LIMIT = 60;

function EmbedEditorModal({
  value,
  onChange,
  previewContent,
  previewAttachments,
  onClose,
}: {
  value: unknown;
  onChange: (value: unknown) => void;
  previewContent: string;
  previewAttachments: Array<{ name: string; size?: string }>;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [doc, setDoc] = useState<EmbedDoc>(() => normalizeEmbedDoc(value));
  const [jsonView, setJsonView] = useState(false);
  const [jsonText, setJsonText] = useState("");
  const [jsonError, setJsonError] = useState<string | null>(null);
  const [historyVersion, setHistoryVersion] = useState(0);
  const docRef = useRef(doc);
  docRef.current = doc;
  const past = useRef<EmbedDoc[]>([]);
  const future = useRef<EmbedDoc[]>([]);

  /* ---------------- history ---------------- */

  const beginHistory = useCallback(() => {
    past.current = [...past.current.slice(-(HISTORY_LIMIT - 1)), docRef.current];
    future.current = [];
    setHistoryVersion((v) => v + 1);
  }, []);

  const undo = useCallback(() => {
    if (past.current.length === 0) return;
    const previous = past.current[past.current.length - 1];
    past.current = past.current.slice(0, -1);
    future.current = [...future.current, docRef.current];
    setDoc(previous);
    docRef.current = previous;
    setHistoryVersion((v) => v + 1);
  }, []);

  const redo = useCallback(() => {
    if (future.current.length === 0) return;
    const next = future.current[future.current.length - 1];
    future.current = future.current.slice(0, -1);
    past.current = [...past.current, docRef.current];
    setDoc(next);
    docRef.current = next;
    setHistoryVersion((v) => v + 1);
  }, []);

  const mutate = useCallback(
    (fn: (current: EmbedDoc) => EmbedDoc, options?: { history?: boolean }) => {
      if (options?.history !== false) beginHistory();
      const next = fn(docRef.current);
      docRef.current = next;
      setDoc(next);
    },
    [beginHistory],
  );

  /* ---------------- external commit ---------------- */

  const committedRef = useRef(false);
  useEffect(() => {
    // skip the mount commit: opening the editor must not mark the graph dirty
    if (!committedRef.current) {
      committedRef.current = true;
      return;
    }
    onChange(serializeEmbedDoc(doc));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [doc]);

  /* ---------------- keyboard ---------------- */

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const mod = event.ctrlKey || event.metaKey;
      if (!mod) return;
      if (event.key.toLowerCase() === "z" && !event.shiftKey) {
        event.preventDefault();
        undo();
      } else if ((event.key.toLowerCase() === "z" && event.shiftKey) || event.key.toLowerCase() === "y") {
        event.preventDefault();
        redo();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [undo, redo]);

  /* ---------------- embed operations ---------------- */

  const patchEmbed = useCallback(
    (id: string, patch: Partial<EmbedSpec>) => {
      mutate(
        (current) => ({
          ...current,
          embeds: current.embeds.map((embed) => (embed.id === id ? { ...embed, ...patch } : embed)),
        }),
        { history: false },
      );
    },
    [mutate],
  );

  const addEmbed = useCallback(() => {
    mutate((current) => {
      if (current.embeds.length >= 10) return current;
      const embed = createEmbed();
      embed.id = nextEmbedId(current.embeds.map((item) => item.id));
      return { ...current, embeds: [...current.embeds, embed] };
    });
  }, [mutate]);

  const removeEmbed = useCallback(
    (id: string) => mutate((current) => ({ ...current, embeds: current.embeds.filter((embed) => embed.id !== id) })),
    [mutate],
  );

  const moveEmbed = useCallback(
    (id: string, direction: -1 | 1) => {
      mutate((current) => {
        const index = current.embeds.findIndex((embed) => embed.id === id);
        const target = index + direction;
        if (index < 0 || target < 0 || target >= current.embeds.length) return current;
        const embeds = [...current.embeds];
        const [item] = embeds.splice(index, 1);
        embeds.splice(target, 0, item);
        return { ...current, embeds };
      });
    },
    [mutate],
  );

  const duplicateEmbed = useCallback(
    (id: string) => {
      mutate((current) => {
        if (current.embeds.length >= 10) return current;
        const source = current.embeds.find((embed) => embed.id === id);
        if (!source) return current;
        const clone: EmbedSpec = {
          ...source,
          id: nextEmbedId(current.embeds.map((item) => item.id)),
          author: { ...source.author },
          footer: { ...source.footer },
          image: { ...source.image },
          thumbnail: { ...source.thumbnail },
          fields: source.fields.map((field, index) => ({
            ...field,
            id: `field_${index + 1}_${Date.now().toString(36).slice(-4)}`,
          })),
        };
        const index = current.embeds.findIndex((embed) => embed.id === id);
        const embeds = [...current.embeds];
        embeds.splice(index + 1, 0, clone);
        return { ...current, embeds };
      });
    },
    [mutate],
  );

  /* ---------------- field operations ---------------- */

  const patchField = useCallback(
    (embedId: string, fieldId: string, patch: Partial<EmbedFieldSpec>) => {
      mutate(
        (current) => ({
          ...current,
          embeds: current.embeds.map((embed) =>
            embed.id === embedId
              ? { ...embed, fields: embed.fields.map((field) => (field.id === fieldId ? { ...field, ...patch } : field)) }
              : embed,
          ),
        }),
        { history: false },
      );
    },
    [mutate],
  );

  const addField = useCallback(
    (embedId: string) => {
      mutate((current) => ({
        ...current,
        embeds: current.embeds.map((embed) => {
          if (embed.id !== embedId || embed.fields.length >= 25) return embed;
          const field = createField();
          field.id = nextFieldId(embed.fields.map((item) => item.id));
          return { ...embed, fields: [...embed.fields, field] };
        }),
      }));
    },
    [mutate],
  );

  const removeField = useCallback(
    (embedId: string, fieldId: string) => {
      mutate((current) => ({
        ...current,
        embeds: current.embeds.map((embed) =>
          embed.id === embedId ? { ...embed, fields: embed.fields.filter((field) => field.id !== fieldId) } : embed,
        ),
      }));
    },
    [mutate],
  );

  const moveField = useCallback(
    (embedId: string, fieldId: string, direction: -1 | 1) => {
      mutate((current) => ({
        ...current,
        embeds: current.embeds.map((embed) => {
          if (embed.id !== embedId) return embed;
          const index = embed.fields.findIndex((field) => field.id === fieldId);
          const target = index + direction;
          if (index < 0 || target < 0 || target >= embed.fields.length) return embed;
          const fields = [...embed.fields];
          const [item] = fields.splice(index, 1);
          fields.splice(target, 0, item);
          return { ...embed, fields };
        }),
      }));
    },
    [mutate],
  );

  const duplicateField = useCallback(
    (embedId: string, fieldId: string) => {
      mutate((current) => ({
        ...current,
        embeds: current.embeds.map((embed) => {
          if (embed.id !== embedId || embed.fields.length >= 25) return embed;
          const source = embed.fields.find((field) => field.id === fieldId);
          if (!source) return embed;
          const clone: EmbedFieldSpec = { ...source, id: nextFieldId(embed.fields.map((item) => item.id)) };
          const index = embed.fields.findIndex((field) => field.id === fieldId);
          const fields = [...embed.fields];
          fields.splice(index + 1, 0, clone);
          return { ...embed, fields };
        }),
      }));
    },
    [mutate],
  );

  /* ---------------- variable operations ---------------- */

  const addPin = useCallback(() => {
    mutate((current) => {
      let name = "var1";
      let counter = 1;
      const taken = new Set(current.pins.map((pin) => pin.name));
      while (taken.has(name)) {
        counter += 1;
        name = `var${counter}`;
      }
      return { ...current, pins: [...current.pins, { name, type: "text", sample: "", default: "" }] };
    });
  }, [mutate]);

  const patchPin = useCallback(
    (name: string, patch: Partial<EmbedPin>) => {
      mutate(
        (current) => ({
          ...current,
          pins: current.pins.map((pin) => (pin.name === name ? { ...pin, ...patch } : pin)),
        }),
        { history: false },
      );
    },
    [mutate],
  );

  const deletePin = useCallback(
    (name: string) => {
      mutate((current) => ({ ...current, pins: current.pins.filter((pin) => pin.name !== name) }));
    },
    [mutate],
  );

  const renamePin = useCallback(
    (oldName: string, newName: string) => {
      mutate((current) => {
        const from = new RegExp(`\\{\\{\\s*${escapeRegExp(oldName)}\\s*\\}\}`, "g");
        const to = `{{${newName}}}`;
        const renameText = (text: string) => text.replace(from, to);
        return {
          ...current,
          pins: current.pins.map((pin) => (pin.name === oldName ? { ...pin, name: newName } : pin)),
          embeds: current.embeds.map((embed) => ({
            ...embed,
            title: renameText(embed.title),
            description: renameText(embed.description),
            author: { ...embed.author, name: renameText(embed.author.name) },
            footer: { ...embed.footer, text: renameText(embed.footer.text) },
            fields: embed.fields.map((field) => ({
              ...field,
              name: renameText(field.name),
              value: renameText(field.value),
            })),
          })),
        };
      });
    },
    [mutate],
  );

  /* ---------------- JSON view ---------------- */

  const openJsonView = useCallback(() => {
    // raw templates, so Apply on an untouched view is a lossless round-trip
    setJsonText(embedsToRawJSON(doc.embeds, {}));
    setJsonError(null);
    setJsonView(true);
  }, [doc]);

  const applyJson = useCallback(() => {
    const parsed = embedsFromRawJSON(jsonText);
    if (!parsed) {
      setJsonError(t("embedEditor.jsonInvalid"));
      return;
    }
    setJsonError(null);
    mutate((current) => ({ ...current, embeds: parsed }));
    setJsonView(false);
  }, [jsonText, mutate, t]);

  const copyJson = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(jsonText);
    } catch {
      // clipboard may be unavailable in the webview; the textarea stays selected
    }
  }, [jsonText]);

  const canUndo = past.current.length > 0;
  const canRedo = future.current.length > 0;
  void historyVersion; // re-render refreshes the toolbar's enabled state

  return (
    <Modal
      title={t("embedEditor.editorTitle")}
      icon="Braces"
      size="full"
      onClose={onClose}
      headerExtra={
        <Badge tone="muted">
          {doc.embeds.length} {t("embedEditor.embedCount")} · {doc.pins.length} {t("embedEditor.variableCount")}
        </Badge>
      }
      toolbar={
        <div className="flex items-center gap-1">
          <IconButton label={t("embedEditor.undo")} icon="Undo2" disabled={!canUndo} onClick={undo} />
          <IconButton label={t("embedEditor.redo")} icon="Redo2" disabled={!canRedo} onClick={redo} />
          <span className="mx-1 h-4 w-px bg-ink-700" />
          <IconButton
            label={jsonView ? t("embedEditor.closeJson") : t("embedEditor.openJson")}
            icon="Braces"
            active={jsonView}
            onClick={() => (jsonView ? setJsonView(false) : openJsonView())}
          />
        </div>
      }
      bodyClassName="min-h-0 flex-1 overflow-hidden p-0"
      footer={
        <div className="flex w-full items-center gap-3">
          <span className="text-[10.5px] text-fg-faint">{t("embedEditor.footerHint")}</span>
          <div className="ml-auto flex items-center gap-2">
            <Button variant="ghost" onClick={onClose}>
              {t("common.cancel")}
            </Button>
            <Button variant="solid" icon="Check" onClick={onClose}>
              {t("embedEditor.done")}
            </Button>
          </div>
        </div>
      }
    >
      <div className="flex h-full min-h-0">
        {jsonView ? (
          <div className="flex min-h-0 w-1/2 flex-col gap-2 border-r border-ink-700/70 p-3">
            <p className="text-[11px] leading-5 text-fg-faint">{t("embedEditor.jsonHint")}</p>
            <textarea
              value={jsonText}
              spellCheck={false}
              onChange={(event) => setJsonText(event.target.value)}
              className={`muted-scroll min-h-0 flex-1 resize-none rounded border bg-ink-950 p-2.5 font-mono text-[11.5px] leading-[1.55] text-fg outline-none transition focus:border-ink-500 ${
                jsonError ? "border-danger/70" : "border-ink-700"
              }`}
            />
            {jsonError ? <p className="text-[11px] text-danger">{jsonError}</p> : null}
            <div className="flex items-center justify-end gap-2">
              <Button variant="ghost" icon="Copy" onClick={copyJson}>
                {t("embedEditor.copyJson")}
              </Button>
              <Button variant="solid" icon="Check" onClick={applyJson}>
                {t("embedEditor.applyJson")}
              </Button>
            </div>
          </div>
        ) : (
          <div className="muted-scroll min-h-0 w-1/2 overflow-y-auto border-r border-ink-700/70 p-3">
            <EmbedForm
              doc={doc}
              onPatchEmbed={patchEmbed}
              onAddEmbed={addEmbed}
              onRemoveEmbed={removeEmbed}
              onMoveEmbed={moveEmbed}
              onDuplicateEmbed={duplicateEmbed}
              onPatchField={patchField}
              onAddField={addField}
              onRemoveField={removeField}
              onMoveField={moveField}
              onDuplicateField={duplicateField}
              onAddPin={addPin}
              onPatchPin={patchPin}
              onDeletePin={deletePin}
              onRenamePin={renamePin}
            />
          </div>
        )}
        <div className="min-h-0 w-1/2 bg-[#313338]">
          <div className="flex items-center gap-2 border-b border-black/30 bg-[#2f3136] px-4 py-2">
            <svg viewBox="0 0 24 24" fill="none" stroke="#b9bbbe" strokeWidth="1.75" className="h-3.5 w-3.5">
              <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
            <span className="text-[11px] font-semibold uppercase tracking-[0.1em] text-[#b9bbbe]">
              {t("embedEditor.preview")}
            </span>
          </div>
          <div className="h-[calc(100%-37px)]">
            <DiscordPreview doc={doc} content={previewContent} attachments={previewAttachments} />
          </div>
        </div>
      </div>
    </Modal>
  );
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
