import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "../ui";
import { Dropdown } from "../Dropdown";
import { TextInput } from "../primitives/Field";
import {
  EMBED_LIMITS,
  type EmbedDoc,
  type EmbedFieldSpec,
  type EmbedPin,
  type EmbedPinType,
  type EmbedSpec,
  isValidEmbedPinName,
  nextEmbedId,
  nextFieldId,
} from "@/lib/embed";
import { CardIconButton, ColorField, ICON_PATHS, Section, TextAreaField, TextField, TimestampField } from "./shared";

export interface EmbedFormProps {
  doc: EmbedDoc;
  onPatchEmbed: (id: string, patch: Partial<EmbedSpec>) => void;
  onAddEmbed: () => void;
  onRemoveEmbed: (id: string) => void;
  onMoveEmbed: (id: string, direction: -1 | 1) => void;
  onDuplicateEmbed: (id: string) => void;
  onPatchField: (embedId: string, fieldId: string, patch: Partial<EmbedFieldSpec>) => void;
  onAddField: (embedId: string) => void;
  onRemoveField: (embedId: string, fieldId: string) => void;
  onMoveField: (embedId: string, fieldId: string, direction: -1 | 1) => void;
  onDuplicateField: (embedId: string, fieldId: string) => void;
  onAddPin: () => void;
  onPatchPin: (name: string, patch: Partial<EmbedPin>) => void;
  onDeletePin: (name: string) => void;
  onRenamePin: (oldName: string, newName: string) => void;
}

export function EmbedForm(props: EmbedFormProps) {
  const { t } = useTranslation();
  const { doc } = props;
  const totalChars = doc.embeds.reduce(
    (total, embed) =>
      total +
      embed.title.length +
      embed.description.length +
      embed.author.name.length +
      embed.footer.text.length +
      embed.fields.reduce((sum, field) => sum + field.name.length + field.value.length, 0),
    0,
  );

  return (
    <div className="space-y-3">
      <Section title={t("embedEditor.variables")} meta={`${doc.pins.length}`} defaultOpen={doc.pins.length === 0}>
        <VariablesPanel {...props} />
      </Section>

      <div className="flex items-center justify-between">
        <span className="text-[10px] font-semibold uppercase tracking-[0.12em] text-fg-faint">
          {t("embedEditor.embeds")}
        </span>
        <span
          className={`text-[10px] tabular-nums ${
            doc.embeds.length > EMBED_LIMITS.embeds || totalChars > EMBED_LIMITS.total ? "font-semibold text-danger" : "text-fg-faint"
          }`}
        >
          {doc.embeds.length} / {EMBED_LIMITS.embeds} · {totalChars} / {EMBED_LIMITS.total}
        </span>
      </div>

      {doc.embeds.length === 0 ? (
        <p className="rounded-md border border-ink-700 bg-ink-950/50 p-3 text-[11px] leading-5 text-fg-faint">
          {t("embedEditor.embedsEmpty")}
        </p>
      ) : (
        doc.embeds.map((embed, index) => (
          <EmbedCardEditor
            key={embed.id}
            embed={embed}
            index={index}
            count={doc.embeds.length}
            {...props}
          />
        ))
      )}

      <Button
        variant="solid"
        icon="Plus"
        className="w-full"
        disabled={doc.embeds.length >= EMBED_LIMITS.embeds}
        onClick={props.onAddEmbed}
      >
        {t("embedEditor.addEmbed")}
      </Button>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* variables                                                           */
/* ------------------------------------------------------------------ */

function VariablesPanel({ doc, onAddPin, onPatchPin, onDeletePin, onRenamePin }: EmbedFormProps) {
  const { t } = useTranslation();
  return (
    <div className="space-y-2">
      {doc.pins.length === 0 ? (
        <p className="text-[11px] leading-5 text-fg-faint">{t("embedEditor.variablesEmpty")}</p>
      ) : (
        doc.pins.map((pin) => (
          <PinRow key={pin.name} pin={pin} doc={doc} onPatchPin={onPatchPin} onDeletePin={onDeletePin} onRenamePin={onRenamePin} />
        ))
      )}
      <Button variant="ghost" icon="Plus" className="h-6 w-full px-2 text-[11px]" onClick={onAddPin}>
        {t("embedEditor.addVariable")}
      </Button>
    </div>
  );
}

function PinRow({
  pin,
  doc,
  onPatchPin,
  onDeletePin,
  onRenamePin,
}: {
  pin: EmbedPin;
  doc: EmbedDoc;
  onPatchPin: (name: string, patch: Partial<EmbedPin>) => void;
  onDeletePin: (name: string) => void;
  onRenamePin: (oldName: string, newName: string) => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState(pin.name);
  const [error, setError] = useState<string | null>(null);

  const commitName = () => {
    const trimmed = name.trim();
    if (trimmed === pin.name) {
      setName(pin.name);
      setError(null);
      return;
    }
    if (!isValidEmbedPinName(trimmed)) {
      setError(t("embedEditor.invalidVariableName"));
      setName(pin.name);
      return;
    }
    if (doc.pins.some((candidate) => candidate.name === trimmed)) {
      setError(t("embedEditor.duplicateVariableName"));
      setName(pin.name);
      return;
    }
    setError(null);
    onRenamePin(pin.name, trimmed);
  };

  return (
    <div className="space-y-1.5 rounded-md border border-ink-700 bg-ink-950/50 p-2.5">
      <div className="flex items-center gap-1.5">
        <input
          value={name}
          onChange={(event) => setName(event.target.value)}
          onBlur={commitName}
          onKeyDown={(event) => {
            if (event.key === "Enter") (event.target as HTMLInputElement).blur();
          }}
          spellCheck={false}
          className={`min-w-0 flex-1 rounded border bg-ink-950 px-2 py-1 font-mono text-[11.5px] text-fg outline-none transition ${
            error ? "border-danger" : "border-ink-700 focus:border-ink-500"
          }`}
        />
        <Dropdown
          compact
          className="w-[86px] shrink-0"
          value={pin.type}
          onChange={(type) => onPatchPin(pin.name, { type: type as EmbedPinType })}
          options={[
            { value: "text", label: t("embedEditor.pinText") },
            { value: "number", label: t("embedEditor.pinNumber") },
            { value: "boolean", label: t("embedEditor.pinBoolean") },
          ]}
        />
        <CardIconButton title={t("embedEditor.deleteVariable")} danger onClick={() => onDeletePin(pin.name)} path={ICON_PATHS.trash} />
      </div>
      {error ? <p className="px-0.5 text-[10px] text-danger">{error}</p> : null}
      <div className="grid grid-cols-2 gap-1.5">
        <label className="block">
          <span className="mb-0.5 block text-[9.5px] font-medium uppercase tracking-wide text-fg-faint">
            {t("embedEditor.sampleValue")}
          </span>
          <TextInput
            value={pin.sample}
            placeholder={pin.type === "number" ? "21" : pin.type === "boolean" ? "true" : "Berlin"}
            onChange={(sample) => onPatchPin(pin.name, { sample })}
          />
        </label>
        <label className="block">
          <span className="mb-0.5 block text-[9.5px] font-medium uppercase tracking-wide text-fg-faint">
            {t("embedEditor.defaultValue")}
          </span>
          <TextInput
            value={pin.default}
            placeholder={t("embedEditor.none")}
            onChange={(defaultValue) => onPatchPin(pin.name, { default: defaultValue })}
          />
        </label>
      </div>
      <div className="flex items-center justify-end">
        <code className="rounded bg-ink-900 px-1.5 py-0.5 font-mono text-[10px] text-fg-subtle">{`{{${pin.name}}}`}</code>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* embed card                                                          */
/* ------------------------------------------------------------------ */

function EmbedCardEditor({
  embed,
  index,
  count,
  onPatchEmbed,
  onRemoveEmbed,
  onMoveEmbed,
  onDuplicateEmbed,
  onPatchField,
  onAddField,
  onRemoveField,
  onMoveField,
  onDuplicateField,
}: EmbedFormProps & { embed: EmbedSpec; index: number; count: number }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(true);
  const subtitle = embed.author.name || embed.title || embed.description.slice(0, 40) || t("embedEditor.emptyEmbedCard");

  return (
    <div
      className="overflow-hidden rounded-md border border-ink-700 bg-ink-850"
      style={{ borderLeft: `4px solid ${embed.color || "#1f2225"}` }}
    >
      <div className="flex items-center gap-1 px-2 py-1.5">
        <button type="button" onClick={() => setOpen(!open)} className="flex min-w-0 flex-1 items-center gap-2 text-left">
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            className={`h-3 w-3 shrink-0 text-fg-faint transition-transform ${open ? "rotate-90" : ""}`}
          >
            <path d="M9 18l6-6-6-6" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          <span className="truncate text-[12px] font-medium text-fg">
            {t("embedEditor.embedNumber", { index: index + 1 })}
          </span>
          <span className="truncate text-[11px] text-fg-faint">{subtitle}</span>
        </button>
        <CardIconButton title={t("embedEditor.moveUp")} disabled={index === 0} onClick={() => onMoveEmbed(embed.id, -1)} path={ICON_PATHS.up} />
        <CardIconButton
          title={t("embedEditor.moveDown")}
          disabled={index === count - 1}
          onClick={() => onMoveEmbed(embed.id, 1)}
          path={ICON_PATHS.down}
        />
        <CardIconButton title={t("embedEditor.duplicate")} onClick={() => onDuplicateEmbed(embed.id)} path={ICON_PATHS.copy} />
        <CardIconButton title={t("embedEditor.deleteEmbed")} danger onClick={() => onRemoveEmbed(embed.id)} path={ICON_PATHS.trash} />
      </div>

      {open ? (
        <div className="space-y-2.5 border-t border-ink-700/50 p-2.5">
          <Section title={t("embedEditor.sectionAuthor")}>
            <TextField
              label={t("embedEditor.authorName")}
              value={embed.author.name}
              max={EMBED_LIMITS.authorName}
              onChange={(name) => onPatchEmbed(embed.id, { author: { ...embed.author, name } })}
            />
            <div className="grid grid-cols-2 gap-2">
              <TextField
                label={t("embedEditor.authorUrl")}
                value={embed.author.url}
                max={2048}
                placeholder="https://example.com"
                onChange={(url) => onPatchEmbed(embed.id, { author: { ...embed.author, url } })}
              />
              <TextField
                label={t("embedEditor.authorIcon")}
                value={embed.author.iconUrl}
                max={2048}
                placeholder="https://example.com/icon.png"
                onChange={(iconUrl) => onPatchEmbed(embed.id, { author: { ...embed.author, iconUrl } })}
              />
            </div>
          </Section>

          <Section title={t("embedEditor.sectionBody")}>
            <TextField
              label={t("embedEditor.title")}
              value={embed.title}
              max={EMBED_LIMITS.title}
              onChange={(title) => onPatchEmbed(embed.id, { title })}
            />
            <TextAreaField
              label={t("embedEditor.description")}
              value={embed.description}
              max={EMBED_LIMITS.description}
              placeholder={t("embedEditor.descriptionPlaceholder")}
              rows={4}
              onChange={(description) => onPatchEmbed(embed.id, { description })}
            />
            <div className="grid grid-cols-2 gap-2">
              <TextField
                label={t("embedEditor.embedUrl")}
                value={embed.url}
                max={2048}
                placeholder="https://example.com"
                onChange={(url) => onPatchEmbed(embed.id, { url })}
              />
              <ColorField
                label={t("embedEditor.color")}
                value={embed.color}
                onChange={(color) => onPatchEmbed(embed.id, { color })}
              />
            </div>
          </Section>

          <Section title={t("embedEditor.sectionImages")}>
            <TextField
              label={t("embedEditor.imageUrl")}
              value={embed.image.url}
              max={2048}
              placeholder="https://example.com/banner.png"
              onChange={(url) => onPatchEmbed(embed.id, { image: { url } })}
            />
            <TextField
              label={t("embedEditor.thumbnailUrl")}
              value={embed.thumbnail.url}
              max={2048}
              placeholder="https://example.com/thumb.png"
              onChange={(url) => onPatchEmbed(embed.id, { thumbnail: { url } })}
            />
          </Section>

          <Section title={t("embedEditor.sectionFooter")}>
            <TextAreaField
              label={t("embedEditor.footerText")}
              value={embed.footer.text}
              max={EMBED_LIMITS.footerText}
              rows={2}
              onChange={(text) => onPatchEmbed(embed.id, { footer: { ...embed.footer, text } })}
            />
            <div className="grid grid-cols-2 gap-2">
              <TextField
                label={t("embedEditor.footerIcon")}
                value={embed.footer.iconUrl}
                max={2048}
                placeholder="https://example.com/icon.png"
                onChange={(iconUrl) => onPatchEmbed(embed.id, { footer: { ...embed.footer, iconUrl } })}
              />
              <TimestampField
                label={t("embedEditor.timestamp")}
                value={embed.timestamp}
                onChange={(timestamp) => onPatchEmbed(embed.id, { timestamp })}
              />
            </div>
          </Section>

          <Section title={t("embedEditor.sectionFields")} meta={`${embed.fields.length} / ${EMBED_LIMITS.fields}`}>
            {embed.fields.length === 0 ? (
              <p className="text-[11px] leading-5 text-fg-faint">{t("embedEditor.fieldsEmpty")}</p>
            ) : (
              embed.fields.map((field, fieldIndex) => (
                <FieldCardEditor
                  key={field.id}
                  embedId={embed.id}
                  field={field}
                  index={fieldIndex}
                  count={embed.fields.length}
                  onPatchField={onPatchField}
                  onRemoveField={onRemoveField}
                  onMoveField={onMoveField}
                  onDuplicateField={onDuplicateField}
                />
              ))
            )}
            <Button
              variant="ghost"
              icon="Plus"
              className="h-6 w-full px-2 text-[11px]"
              disabled={embed.fields.length >= EMBED_LIMITS.fields}
              onClick={() => onAddField(embed.id)}
            >
              {t("embedEditor.addField")}
            </Button>
          </Section>
        </div>
      ) : null}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* field card                                                          */
/* ------------------------------------------------------------------ */

function FieldCardEditor({
  embedId,
  field,
  index,
  count,
  onPatchField,
  onRemoveField,
  onMoveField,
  onDuplicateField,
}: {
  embedId: string;
  field: EmbedFieldSpec;
  index: number;
  count: number;
  onPatchField: (embedId: string, fieldId: string, patch: Partial<EmbedFieldSpec>) => void;
  onRemoveField: (embedId: string, fieldId: string) => void;
  onMoveField: (embedId: string, fieldId: string, direction: -1 | 1) => void;
  onDuplicateField: (embedId: string, fieldId: string) => void;
}) {
  const { t } = useTranslation();
  const patch = (next: Partial<EmbedFieldSpec>) => onPatchField(embedId, field.id, next);

  return (
    <div className="space-y-2 rounded-md border border-ink-700 bg-ink-950/50 p-2.5">
      <div className="flex items-center gap-1.5">
        <span className="flex-1 truncate text-[11px] font-medium text-fg-subtle">
          {t("embedEditor.fieldNumber", { index: index + 1 })}
        </span>
        <label className="flex cursor-pointer items-center gap-1.5 text-[10.5px] text-fg-faint">
          <input
            type="checkbox"
            checked={field.inline}
            onChange={(event) => patch({ inline: event.target.checked })}
            className="h-3 w-3 accent-[var(--status-info)]"
          />
          {t("embedEditor.inline")}
        </label>
        <CardIconButton title={t("embedEditor.moveUp")} disabled={index === 0} onClick={() => onMoveField(embedId, field.id, -1)} path={ICON_PATHS.up} />
        <CardIconButton
          title={t("embedEditor.moveDown")}
          disabled={index === count - 1}
          onClick={() => onMoveField(embedId, field.id, 1)}
          path={ICON_PATHS.down}
        />
        <CardIconButton title={t("embedEditor.duplicate")} onClick={() => onDuplicateField(embedId, field.id)} path={ICON_PATHS.copy} />
        <CardIconButton title={t("embedEditor.deleteField")} danger onClick={() => onRemoveField(embedId, field.id)} path={ICON_PATHS.trash} />
      </div>
      <TextField label={t("embedEditor.fieldName")} value={field.name} max={EMBED_LIMITS.fieldName} onChange={(name) => patch({ name })} />
      <TextAreaField label={t("embedEditor.fieldValue")} value={field.value} max={EMBED_LIMITS.fieldValue} rows={2} onChange={(value) => patch({ value })} />
    </div>
  );
}

/** Re-exported so the main editor can generate fresh ids. */
export { nextEmbedId, nextFieldId };
