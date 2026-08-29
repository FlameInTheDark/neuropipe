/**
 * Discord embed document model — the TypeScript twin of the Go
 * embeddoc.go contract. The editor mutates this shape, the node config
 * persists it, and the backend interpolates and validates it with the same
 * rules mirrored here for live previews and counters.
 */

export type EmbedPinType = "text" | "number" | "boolean";

export interface EmbedPin {
  name: string;
  type: EmbedPinType;
  sample: string;
  default: string;
}

export interface EmbedAuthorSpec {
  name: string;
  url: string;
  iconUrl: string;
}

export interface EmbedFooterSpec {
  text: string;
  iconUrl: string;
}

export interface EmbedMediaSpec {
  url: string;
}

export interface EmbedFieldSpec {
  id: string;
  name: string;
  value: string;
  inline: boolean;
}

export interface EmbedSpec {
  id: string;
  title: string;
  description: string;
  url: string;
  color: string;
  timestamp: string;
  author: EmbedAuthorSpec;
  footer: EmbedFooterSpec;
  image: EmbedMediaSpec;
  thumbnail: EmbedMediaSpec;
  fields: EmbedFieldSpec[];
}

export interface EmbedDoc {
  version: 1;
  pins: EmbedPin[];
  embeds: EmbedSpec[];
}

/* Discord's embed limits, mirrored for live counters. */
export const EMBED_LIMITS = {
  embeds: 10,
  title: 256,
  description: 4096,
  authorName: 256,
  footerText: 2048,
  fields: 25,
  fieldName: 256,
  fieldValue: 1024,
  total: 6000,
} as const;

export function nextEmbedId(used: Iterable<string>): string {
  const taken = new Set(used);
  for (let index = 1; ; index += 1) {
    const id = `embed_${index}`;
    if (!taken.has(id)) return id;
  }
}

export function nextFieldId(used: Iterable<string>): string {
  const taken = new Set(used);
  for (let index = 1; ; index += 1) {
    const id = `field_${index}`;
    if (!taken.has(id)) return id;
  }
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function str(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  if (typeof value === "boolean") return String(value);
  return "";
}

function trimmed(value: unknown): string {
  return str(value).trim();
}

export function normalizeEmbedDoc(value: unknown): EmbedDoc {
  const root = asRecord(value);
  const pins: EmbedPin[] = [];
  if (Array.isArray(root.pins)) {
    for (const entry of root.pins) {
      const pin = asRecord(entry);
      const name = trimmed(pin.name);
      if (!name || pins.length >= 32) continue;
      const type: EmbedPinType =
        pin.type === "number" || pin.type === "boolean" ? pin.type : "text";
      pins.push({ name, type, sample: str(pin.sample), default: str(pin.default) });
    }
  }
  const embeds: EmbedSpec[] = [];
  if (Array.isArray(root.embeds)) {
    for (const entry of root.embeds) {
      const spec = asRecord(entry);
      const author = asRecord(spec.author);
      const footer = asRecord(spec.footer);
      const image = asRecord(spec.image);
      const thumbnail = asRecord(spec.thumbnail);
      const fields: EmbedFieldSpec[] = [];
      if (Array.isArray(spec.fields)) {
        for (const fieldEntry of spec.fields) {
          const field = asRecord(fieldEntry);
          fields.push({
            id: trimmed(field.id) || nextFieldId(fields.map((f) => f.id)),
            name: str(field.name),
            value: str(field.value),
            inline: field.inline === true,
          });
        }
      }
      embeds.push({
        id: trimmed(spec.id) || nextEmbedId(embeds.map((e) => e.id)),
        title: str(spec.title),
        description: str(spec.description),
        url: trimmed(spec.url),
        color: trimmed(spec.color),
        timestamp: trimmed(spec.timestamp),
        author: { name: str(author.name), url: trimmed(author.url), iconUrl: trimmed(author.iconUrl) },
        footer: { text: str(footer.text), iconUrl: trimmed(footer.iconUrl) },
        image: { url: trimmed(image.url) },
        thumbnail: { url: trimmed(thumbnail.url) },
        fields,
      });
    }
  }
  return { version: 1, pins, embeds };
}

export function createEmbed(): EmbedSpec {
  return {
    id: "embed_1",
    title: "",
    description: "",
    url: "",
    color: "#5865F2",
    timestamp: "",
    author: { name: "", url: "", iconUrl: "" },
    footer: { text: "", iconUrl: "" },
    image: { url: "" },
    thumbnail: { url: "" },
    fields: [],
  };
}

export function createField(): EmbedFieldSpec {
  return { id: "field_1", name: "", value: "", inline: false };
}

export function serializeEmbedDoc(doc: EmbedDoc): Record<string, unknown> {
  return {
    version: 1,
    pins: doc.pins.map((pin) => ({
      name: pin.name,
      type: pin.type,
      sample: pin.sample,
      default: pin.default,
    })),
    embeds: doc.embeds.map((embed) => ({
      id: embed.id,
      title: embed.title,
      description: embed.description,
      url: embed.url,
      color: embed.color,
      timestamp: embed.timestamp,
      author: { ...embed.author },
      footer: { ...embed.footer },
      image: { ...embed.image },
      thumbnail: { ...embed.thumbnail },
      fields: embed.fields.map((field) => ({ ...field })),
    })),
  };
}

/** Formats one pin value for interpolation, mirroring Go's FormatValue. */
export function formatEmbedValue(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  if (typeof value === "number") return String(value);
  if (typeof value === "boolean") return value ? "true" : "false";
  if (Array.isArray(value) || typeof value === "object") {
    try {
      return JSON.stringify(value);
    } catch {
      return String(value);
    }
  }
  return String(value);
}

/** Replaces {{name}} references, mirroring Go's Interpolate. */
export function interpolateEmbedText(text: string, values: Record<string, unknown>): string {
  if (!text.includes("{{")) return text;
  return text.replace(/\{\{\s*([^{}]+?)\s*\}\}/g, (_match, name: string) =>
    formatEmbedValue(values[name.trim()]),
  );
}

/** Sample values used by the preview: the pin's sample, then its default. */
export function embedSampleValues(doc: EmbedDoc): Record<string, unknown> {
  const values: Record<string, unknown> = {};
  for (const pin of doc.pins) {
    if (pin.sample !== "") {
      values[pin.name] = pin.sample;
      continue;
    }
    if (pin.default === "") continue;
    if (pin.type === "number") {
      const parsed = Number(pin.default);
      values[pin.name] = Number.isNaN(parsed) ? pin.default : parsed;
    } else if (pin.type === "boolean") {
      values[pin.name] = pin.default === "true";
    } else {
      values[pin.name] = pin.default;
    }
  }
  return values;
}

/** Total embedded character count, mirroring Discord's combined limit. */
export function embedTotalChars(embeds: readonly EmbedSpec[], values: Record<string, unknown>): number {
  return embeds.reduce((total, embed) => {
    let count = 0;
    count += interpolateEmbedText(embed.title, values).length;
    count += interpolateEmbedText(embed.description, values).length;
    count += interpolateEmbedText(embed.author.name, values).length;
    count += interpolateEmbedText(embed.footer.text, values).length;
    for (const field of embed.fields) {
      count += interpolateEmbedText(field.name, values).length;
      count += interpolateEmbedText(field.value, values).length;
    }
    return total + count;
  }, 0);
}

/** Valid identifier for a template variable (also the pin name). */
export function isValidEmbedPinName(name: string): boolean {
  return /^[A-Za-z][A-Za-z0-9_]*$/.test(name) && name.length <= 64;
}

/** Converts a #RRGGBB hex string to Discord's integer color. */
export function embedColorToInt(color: string): number | null {
  const hex = color.trim().replace(/^#/, "");
  if (!/^[0-9a-fA-F]{6}$/.test(hex)) return null;
  return parseInt(hex, 16);
}

export interface RawDiscordEmbed {
  title?: string;
  description?: string;
  url?: string;
  timestamp?: string;
  color?: number;
  footer?: { text?: string; icon_url?: string };
  image?: { url?: string };
  thumbnail?: { url?: string };
  author?: { name?: string; url?: string; icon_url?: string };
  fields?: Array<{ name?: string; value?: string; inline?: boolean }>;
}

/** Converts canonical Discord embed JSON into editor embed specs. */
export function embedsFromRawJSON(raw: string): EmbedSpec[] | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  const list = Array.isArray(parsed) ? parsed : [parsed];
  const embeds: EmbedSpec[] = [];
  for (const entry of list) {
    const rawEmbed = asRecord(entry) as unknown as RawDiscordEmbed;
    if (typeof entry !== "object" || entry === null || Array.isArray(entry)) return null;
    embeds.push({
      id: nextEmbedId(embeds.map((e) => e.id)),
      title: str(rawEmbed.title),
      description: str(rawEmbed.description),
      url: trimmed(rawEmbed.url),
      color:
        typeof rawEmbed.color === "number" && rawEmbed.color >= 0 && rawEmbed.color <= 0xffffff
          ? `#${rawEmbed.color.toString(16).padStart(6, "0")}`
          : "",
      timestamp: trimmed(rawEmbed.timestamp),
      author: {
        name: str(rawEmbed.author?.name),
        url: trimmed(rawEmbed.author?.url),
        iconUrl: trimmed(rawEmbed.author?.icon_url),
      },
      footer: {
        text: str(rawEmbed.footer?.text),
        iconUrl: trimmed(rawEmbed.footer?.icon_url),
      },
      image: { url: trimmed(rawEmbed.image?.url) },
      thumbnail: { url: trimmed(rawEmbed.thumbnail?.url) },
      fields: (rawEmbed.fields ?? []).map((field) => ({
        id: nextFieldId([]),
        name: str(field?.name),
        value: str(field?.value),
        inline: field?.inline === true,
      })),
    });
  }
  return embeds;
}

/** Serializes editor embeds to canonical Discord embed JSON. By default
 * templates stay raw ({{city}}) so the JSON view is a lossless round-trip;
 * set interpolate to true to export ready-to-send JSON with resolved values. */
export function embedsToRawJSON(
  embeds: readonly EmbedSpec[],
  values: Record<string, unknown>,
  options?: { interpolate?: boolean },
): string {
  const resolve = (text: string) =>
    options?.interpolate ? interpolateEmbedText(text, values) : text;
  const payload = embeds.map((embed) => {
    const output: Record<string, unknown> = {};
    const title = resolve(embed.title);
    const description = resolve(embed.description);
    if (title) output.title = title;
    if (description) output.description = description;
    if (embed.url) output.url = embed.url;
    const color = embedColorToInt(embed.color);
    if (color !== null && color !== 0) output.color = color;
    if (embed.timestamp) output.timestamp = embed.timestamp;
    if (embed.author.name) {
      output.author = {
        name: resolve(embed.author.name),
        ...(embed.author.url ? { url: embed.author.url } : {}),
        ...(embed.author.iconUrl ? { icon_url: embed.author.iconUrl } : {}),
      };
    }
    if (embed.footer.text) {
      output.footer = {
        text: resolve(embed.footer.text),
        ...(embed.footer.iconUrl ? { icon_url: embed.footer.iconUrl } : {}),
      };
    }
    if (embed.image.url) output.image = { url: embed.image.url };
    if (embed.thumbnail.url) output.thumbnail = { url: embed.thumbnail.url };
    const fields = embed.fields
      .filter((field) => field.name && field.value)
      .map((field) => ({
        name: resolve(field.name),
        value: resolve(field.value),
        ...(field.inline ? { inline: true } : {}),
      }));
    if (fields.length > 0) output.fields = fields;
    return output;
  });
  return JSON.stringify(payload.length === 1 ? payload[0] : payload, null, 2);
}
