import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  type EmbedSpec,
  embedColorToInt,
  embedSampleValues,
  interpolateEmbedText,
} from "@/lib/embed";
import { PreviewImage, formatEmbedTimestamp } from "./shared";

export interface DiscordPreviewProps {
  doc: import("@/lib/embed").EmbedDoc;
  /** Message content from the node's own message field (already raw text). */
  content: string;
  /** Attachment display names from the node's file pins/fields. */
  attachments: Array<{ name: string; size?: string }>;
  /** Bot display name shown above the message. */
  botName?: string;
}

/**
 * A pixel-faithful replica of Discord's dark chat rendering for embeds,
 * matching merlinfuchs/embed-generator's preview: #36393e chat, #2f3136 embed
 * cards with a 4px colored bar, 12-column inline field grid, and a thumbnail
 * anchored top-right. Templates resolve against the variables' sample values
 * so the preview shows the message as it will look with live data.
 */
export function DiscordPreview({ doc, content, attachments, botName }: DiscordPreviewProps) {
  const { t } = useTranslation();
  const values = useMemo(() => embedSampleValues(doc), [doc]);
  const renderedContent = useMemo(() => interpolateEmbedText(content, values), [content, values]);
  const time = useMemo(() => {
    const now = new Date();
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${t("embedPreview.todayAt")} ${pad(now.getHours())}:${pad(now.getMinutes())}`;
  }, [t]);

  return (
    <div className="discord-preview muted-scroll h-full overflow-y-auto p-4">
      <div className="discord-message">
        <div className="discord-avatar">N</div>
        <div className="discord-message-body">
          <div className="discord-message-header">
            <span className="discord-message-author">{botName || t("embedPreview.botName")}</span>
            <span className="discord-bot-tag">BOT</span>
            <span className="discord-message-time">{time}</span>
          </div>
          {renderedContent ? <div className="discord-message-content">{renderedContent}</div> : null}
          {attachments.length > 0 ? (
            <div className="discord-embeds">
              {attachments.map((attachment, index) => (
                <div key={index} className="discord-attachment-chip">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" className="h-4 w-4 shrink-0">
                    <path d="M21.44 11.05l-9.19 9.19a6 6 0 01-8.49-8.49l9.19-9.19a4 4 0 015.66 5.66l-9.2 9.19a2 2 0 01-2.83-2.83l8.49-8.48" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                  <span className="truncate">{attachment.name}</span>
                  {attachment.size ? <span className="size shrink-0">{attachment.size}</span> : null}
                </div>
              ))}
            </div>
          ) : null}
          {doc.embeds.length > 0 ? (
            <div className="discord-embeds">
              {doc.embeds.map((embed) => (
                <EmbedCard key={embed.id} embed={embed} values={values} />
              ))}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function EmbedCard({ embed, values }: { embed: EmbedSpec; values: Record<string, unknown> }) {
  const { t } = useTranslation();
  const color = embedColorToInt(embed.color);
  const barColor = color !== null && color !== 0 ? `#${(color & 0xffffff).toString(16).padStart(6, "0")}` : "#202225";
  const title = interpolateEmbedText(embed.title, values);
  const description = interpolateEmbedText(embed.description, values);
  const authorName = interpolateEmbedText(embed.author.name, values);
  const footerText = interpolateEmbedText(embed.footer.text, values);
  const hasContent =
    Boolean(embed.url) ||
    Boolean(title) ||
    Boolean(description) ||
    Boolean(authorName) ||
    Boolean(footerText) ||
    embed.image.url !== "" ||
    embed.thumbnail.url !== "" ||
    embed.fields.length > 0;
  const fields = embed.fields.filter((field) => field.name && field.value);
  const fieldsWithColumns = assignFieldColumns(fields);
  const hasRightMedia = embed.thumbnail.url !== "";

  return (
    <div className="discord-embed">
      <div className="discord-embed-bar" style={{ background: barColor }} />
      <div className="discord-embed-wrapper">
        <div className="discord-embed-grid">
          <div className="discord-embed-main">
            {authorName ? (
              <div className="discord-embed-author">
                {embed.author.iconUrl ? (
                  <PreviewImage src={embed.author.iconUrl} className="discord-embed-author-icon" />
                ) : null}
                {embed.author.url ? (
                  <a className="discord-embed-author-name" style={{ color: "#00aff4" }} href={embed.author.url} onClick={(event) => event.preventDefault()}>
                    {authorName}
                  </a>
                ) : (
                  <span className="discord-embed-author-name">{authorName}</span>
                )}
              </div>
            ) : null}
            {title ? (
              embed.url ? (
                <a className="discord-embed-title" style={{ color: "#00aff4" }} href={embed.url} onClick={(event) => event.preventDefault()}>
                  {title}
                </a>
              ) : (
                <div className="discord-embed-title">{title}</div>
              )
            ) : null}
            {description ? <div className="discord-embed-description">{description}</div> : null}
            {fieldsWithColumns.length > 0 ? (
              <div className="discord-embed-fields">
                {fieldsWithColumns.map((field) => (
                  <div key={field.id} className={`discord-embed-field discord-embed-field-${field.column}`}>
                    <div className="discord-embed-field-name">{interpolateEmbedText(field.name, values)}</div>
                    <div className="discord-embed-field-value">{interpolateEmbedText(field.value, values)}</div>
                  </div>
                ))}
              </div>
            ) : null}
            {embed.image.url ? <PreviewImage src={embed.image.url} className="discord-embed-image" /> : null}
            {footerText || embed.timestamp ? (
              <div className="discord-embed-footer">
                {embed.footer.iconUrl ? (
                  <PreviewImage src={embed.footer.iconUrl} className="discord-embed-footer-icon" />
                ) : null}
                <span>{footerText}</span>
                {footerText && embed.timestamp ? <span>•</span> : null}
                {embed.timestamp ? <span>{formatEmbedTimestamp(embed.timestamp)}</span> : null}
              </div>
            ) : null}
            {!hasContent ? (
              <div className="discord-embed-description" style={{ color: "#72767d", fontStyle: "italic" }}>
                {t("embedPreview.emptyEmbed")}
              </div>
            ) : null}
          </div>
          {hasRightMedia ? <PreviewImage src={embed.thumbnail.url} className="discord-embed-thumbnail" /> : null}
        </div>
      </div>
    </div>
  );
}

/**
 * Discord lays inline fields three per row at fixed columns (1/5, 5/9, 9/13
 * of a 12-column grid); a non-inline field takes a full row. The running
 * index counts only inline fields, exactly like embed-generator's preview.
 */
function assignFieldColumns(
  fields: Array<{ id: string; name: string; value: string; inline: boolean }>,
): Array<{ id: string; name: string; value: string; inline: boolean; column: string }> {
  let inlineIndex = 0;
  return fields.map((field) => {
    if (!field.inline) {
      return { ...field, column: "full" };
    }
    inlineIndex += 1;
    return { ...field, column: String(((inlineIndex - 1) % 3) + 1) };
  });
}
