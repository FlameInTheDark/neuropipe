import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { LocalDeck } from "@/views/BoardView";
import { Modal, ModalActions } from "../../components/primitives/Modal";
import { Field, TextInput } from "../../components/primitives/Field";
import { IconPickerGrid } from "../../components/primitives/IconPickerGrid";
import { cn } from "../../utils/cn";

/** Rename the local display label of a board key (binding stays backend-owned). */
export function KeyEditor({
  value,
  onSave,
  onCancel,
}: {
  value: { id: string; displayLabel: string };
  onSave: (label: string) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const [label, setLabel] = useState(value.displayLabel);

  return (
    <Modal
      title={t("board.editKey")}
      icon="Pencil"
      onClose={onCancel}
      footer={
        <>
          <span className="text-[11px] text-fg-faint">{t("board.keyHint")}</span>
          <ModalActions
            onCancel={onCancel}
            onConfirm={() => onSave(label.trim())}
            confirmLabel={t("common.save")}
            disabled={!label.trim()}
          />
        </>
      }
    >
      <Field label={t("board.keyLabel")}>
        <TextInput autoFocus value={label} onChange={setLabel} placeholder={t("board.keyPlaceholder")} />
      </Field>
      <p className="text-[11px] leading-relaxed text-fg-faint">{t("board.keyBackendNote")}</p>
    </Modal>
  );
}

/** Rename / re-icon / delete a locally grouped deck. */
export function DeckEditor({
  value,
  canDelete,
  onSave,
  onDelete,
  onCancel,
}: {
  value: LocalDeck;
  canDelete: boolean;
  onSave: (v: LocalDeck) => void;
  onDelete: () => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(value);

  return (
    <Modal
      title={t("board.deckSettings")}
      icon={draft.icon}
      onClose={onCancel}
      footer={
        <>
          <button
            onClick={onDelete}
            disabled={!canDelete}
            className={cn(
              "h-7 rounded-md border px-3 text-[11.5px] transition",
              canDelete
                ? "border-danger/30 bg-danger/10 text-danger-fg hover:bg-danger/20"
                : "cursor-not-allowed border-ink-700 bg-ink-850 text-fg-faint",
            )}
          >
            {t("board.deleteDeck")}
          </button>
          <ModalActions onCancel={onCancel} onConfirm={() => onSave(draft)} disabled={!draft.name.trim()} />
        </>
      }
    >
      <Field label={t("board.deckName")}>
        <TextInput autoFocus value={draft.name} onChange={(name) => setDraft((d) => ({ ...d, name }))} />
      </Field>
      <Field label={t("board.deckIcon")}>
        <IconPickerGrid value={draft.icon} onChange={(icon) => setDraft((d) => ({ ...d, icon }))} />
      </Field>
      <p className="text-[11px] leading-relaxed text-fg-faint">{t("board.deckLocalNote")}</p>
    </Modal>
  );
}
