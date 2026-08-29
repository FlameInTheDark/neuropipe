/* Unit tests for the send-node image/file source gating: the visibleWhen
   predicate engine (adapters) and the config-driven pin filtering twins
   (blueprint-dynamic-pins) for Discord Send Message, Telegram Send Photo,
   and Telegram Send Document.
   Run: npx tsx scripts/test-send-source-gating.mts */

import "./dom-stub.mts";
import { visibleFields } from "../src/lib/adapters";
import type { ConfigField } from "../src/lib/types";
import { resolveConfigDrivenInputs } from "../src/lib/blueprint-dynamic-pins";
import type { NodeDefinition, NodePort } from "../src/lib/types";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

/* ------------------------------------------------------------------ */
/* visibleWhen predicate semantics                                     */
/* ------------------------------------------------------------------ */

const field = (name: string, visibleWhen?: string): ConfigField => ({
  name,
  label: name,
  kind: "string",
  visibleWhen,
});

const keys = (fields: ConfigField[], values: Record<string, unknown>) =>
  visibleFields(fields, values).map((f) => f.name);

const gatingFields: ConfigField[] = [
  field("imageSource"),
  field("fileUrl", "imageSource=url|imageSource="),
  field("filePath", "imageSource=file|imageSource="),
  field("fileBase64", "imageSource=base64|imageSource="),
  field("fileName", "imageSource=base64|imageSource=bytes|imageSource="),
];

check(
  "auto mode (explicit empty) shows every source field",
  JSON.stringify(keys(gatingFields, { imageSource: "" })) ===
    JSON.stringify(["imageSource", "fileUrl", "filePath", "fileBase64", "fileName"]),
);
check(
  "legacy graph (missing key) reads as auto",
  JSON.stringify(keys(gatingFields, {})) ===
    JSON.stringify(["imageSource", "fileUrl", "filePath", "fileBase64", "fileName"]),
);
check(
  "url mode shows only the URL field",
  JSON.stringify(keys(gatingFields, { imageSource: "url" })) ===
    JSON.stringify(["imageSource", "fileUrl"]),
);
check(
  "base64 mode shows base64 + name",
  JSON.stringify(keys(gatingFields, { imageSource: "base64" })) ===
    JSON.stringify(["imageSource", "fileBase64", "fileName"]),
);
check(
  "bytes mode shows name but no url/path/base64 fields",
  JSON.stringify(keys(gatingFields, { imageSource: "bytes" })) ===
    JSON.stringify(["imageSource", "fileName"]),
);
check(
  "file mode shows only the path field",
  JSON.stringify(keys(gatingFields, { imageSource: "file" })) ===
    JSON.stringify(["imageSource", "filePath"]),
);

/* truthiness + negation + != still behave (regression guard for existing nodes) */
check("truthy predicate", keys([field("a"), field("b", "a")], { a: true }).includes("b"));
check("negated predicate hides on truthy", !keys([field("b", "!a")], { a: true }).includes("b"));
check("negated predicate shows on falsy", keys([field("b", "!a")], { a: false }).includes("b"));
check("inequality predicate", keys([field("b", "mode!=url")], { mode: "file" }).includes("b"));
check("inequality predicate excludes match", !keys([field("b", "mode!=url")], { mode: "url" }).includes("b"));
check(
  "chatMode legacy special case still maps to history",
  keys([field("chatId", "chatMode")], { chatMode: "history" }).includes("chatId") &&
    !keys([field("chatId", "chatMode")], { chatMode: "message" }).includes("chatId"),
);
check("OR list with undefined matching empty", keys([field("b", "x=|x=url")], {}).includes("b"));

/* ------------------------------------------------------------------ */
/* pin filtering twins                                                 */
/* ------------------------------------------------------------------ */

const pin = (id: string): NodePort => ({
  id,
  label: id,
  kind: "data",
  direction: "input",
  dataType: "text",
  color: "#fff",
  maxConnections: 1,
});

const discordDefinition: NodeDefinition = {
  type: "action:discord_send_message",
  label: "Send Discord Message",
  category: "Discord",
  inputs: [
    { ...pin("in"), kind: "exec" },
    pin("message"),
    pin("channel"),
    pin("replyToMessageId"),
    pin("embedsJson"),
    pin("fileUrl"),
    pin("filePath"),
    pin("fileBase64"),
    { ...pin("fileData"), dataType: "bytes" },
    pin("fileName"),
    pin("identityId"),
  ],
  outputs: [],
  fields: [],
  defaultConfig: {
    imageSource: "",
    fileUrl: "",
    filePath: "",
    fileBase64: "",
    fileName: "",
    embeds: { pins: [], embeds: [] },
  },
};

const photoDefinition: NodeDefinition = {
  type: "action:telegram_send_photo",
  label: "Send Telegram Photo",
  category: "Telegram",
  inputs: [
    { ...pin("in"), kind: "exec" },
    pin("photoUrl"),
    pin("photoPath"),
    pin("photoBase64"),
    { ...pin("photoData"), dataType: "bytes" },
    pin("photoName"),
    pin("caption"),
    pin("chatId"),
    pin("parseMode"),
    pin("identityId"),
  ],
  outputs: [],
  fields: [],
  defaultConfig: { photoSource: "", photoUrl: "", photoPath: "", photoBase64: "", photoName: "" },
};

const documentDefinition: NodeDefinition = {
  type: "action:telegram_send_document",
  label: "Send Telegram Document",
  category: "Telegram",
  inputs: [
    { ...pin("in"), kind: "exec" },
    pin("documentUrl"),
    pin("documentPath"),
    pin("documentBase64"),
    { ...pin("documentData"), dataType: "bytes" },
    pin("fileName"),
    pin("caption"),
    pin("chatId"),
    pin("parseMode"),
    pin("replyToMessageId"),
    pin("disableNotification"),
    pin("identityId"),
  ],
  outputs: [],
  fields: [],
  defaultConfig: { documentSource: "", documentUrl: "", documentPath: "", documentBase64: "", fileName: "" },
};

const ids = (definition: NodeDefinition, config: Record<string, unknown>) =>
  resolveConfigDrivenInputs(definition, config).map((p) => p.id);

const discordAll = ["in", "message", "channel", "replyToMessageId", "embedsJson", "fileUrl", "filePath", "fileBase64", "fileData", "fileName", "identityId"];
check(
  "discord auto keeps every pin",
  JSON.stringify(ids(discordDefinition, {})) === JSON.stringify(discordAll),
  ids(discordDefinition, {}).join(","),
);
check(
  "discord url mode keeps only fileUrl",
  JSON.stringify(ids(discordDefinition, { imageSource: "url" })) ===
    JSON.stringify(["in", "message", "channel", "replyToMessageId", "embedsJson", "fileUrl", "identityId"]),
  ids(discordDefinition, { imageSource: "url" }).join(","),
);
check(
  "discord bytes mode keeps fileData + fileName",
  JSON.stringify(ids(discordDefinition, { imageSource: "bytes" })) ===
    JSON.stringify(["in", "message", "channel", "replyToMessageId", "embedsJson", "fileData", "fileName", "identityId"]),
  ids(discordDefinition, { imageSource: "bytes" }).join(","),
);
check(
  "discord base64 mode keeps fileBase64 + fileName",
  JSON.stringify(ids(discordDefinition, { imageSource: "base64" })) ===
    JSON.stringify(["in", "message", "channel", "replyToMessageId", "embedsJson", "fileBase64", "fileName", "identityId"]),
);
check(
  "discord file mode keeps only filePath (no name)",
  JSON.stringify(ids(discordDefinition, { imageSource: "file" })) ===
    JSON.stringify(["in", "message", "channel", "replyToMessageId", "embedsJson", "filePath", "identityId"]),
);
check(
  "discord default config (new node) resolves as auto",
  JSON.stringify(ids(discordDefinition, discordDefinition.defaultConfig as Record<string, unknown>)) ===
    JSON.stringify(discordAll),
);

const photoAll = ["in", "photoUrl", "photoPath", "photoBase64", "photoData", "photoName", "caption", "chatId", "parseMode", "identityId"];
check(
  "photo auto keeps every pin",
  JSON.stringify(ids(photoDefinition, {})) === JSON.stringify(photoAll),
  ids(photoDefinition, {}).join(","),
);
check(
  "photo url mode keeps only photoUrl",
  JSON.stringify(ids(photoDefinition, { photoSource: "url" })) ===
    JSON.stringify(["in", "photoUrl", "caption", "chatId", "parseMode", "identityId"]),
  ids(photoDefinition, { photoSource: "url" }).join(","),
);
check(
  "photo bytes mode keeps photoData + photoName",
  JSON.stringify(ids(photoDefinition, { photoSource: "bytes" })) ===
    JSON.stringify(["in", "photoData", "photoName", "caption", "chatId", "parseMode", "identityId"]),
);
check(
  "photo base64 mode keeps photoBase64 + photoName",
  JSON.stringify(ids(photoDefinition, { photoSource: "base64" })) ===
    JSON.stringify(["in", "photoBase64", "photoName", "caption", "chatId", "parseMode", "identityId"]),
);

const documentAll = ["in", "documentUrl", "documentPath", "documentBase64", "documentData", "fileName", "caption", "chatId", "parseMode", "replyToMessageId", "disableNotification", "identityId"];
check(
  "document auto keeps every pin",
  JSON.stringify(ids(documentDefinition, {})) === JSON.stringify(documentAll),
  ids(documentDefinition, {}).join(","),
);
check(
  "document file mode keeps only documentPath",
  JSON.stringify(ids(documentDefinition, { documentSource: "file" })) ===
    JSON.stringify(["in", "documentPath", "caption", "chatId", "parseMode", "replyToMessageId", "disableNotification", "identityId"]),
);
check(
  "document base64 mode keeps documentBase64 + fileName",
  JSON.stringify(ids(documentDefinition, { documentSource: "base64" })) ===
    JSON.stringify(["in", "documentBase64", "fileName", "caption", "chatId", "parseMode", "replyToMessageId", "disableNotification", "identityId"]),
);
check(
  "unknown mode value reads as auto (typo safety)",
  JSON.stringify(ids(photoDefinition, { photoSource: "wat" })) === JSON.stringify(photoAll),
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);

/* ------------------------------------------------------------------ */
/* part 2: pin the backend wiring so a refactor cannot drop it         */
/* ------------------------------------------------------------------ */

import { readFileSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..", "..");
const go = (relative: string) => readFileSync(path.join(root, relative), "utf8");

function pins(name: string, ok: boolean) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}`);
  if (!ok) process.exitCode = 1;
}

const discord = go("internal/nodes/discord/sendmessage/sendmessage.go");
pins(
  "discord node declares the imageSource select with shared options",
  discord.includes(`Name: "imageSource", Label: "Image source", Kind: "select", Options: attachments.SourceOptions()`) &&
    discord.includes(`"imageSource": ""`),
);
pins(
  "discord node declares the fileBase64 pin and field",
  discord.includes(`dc.Text("fileBase64", "File base64", domain.PinInput, false)`) &&
    discord.includes(`sourceField("fileBase64", "File base64", "textarea"`),
);
pins(
  "discord resolver filters attachment pins by source mode",
  discord.includes("filterImagePins") && discord.includes(`attachments.SourceMode(configValue(node, "imageSource"))`),
);

const photo = go("internal/nodes/telegram/sendphoto/sendphoto.go");
pins(
  "photo node declares the photoSource select and default",
  photo.includes(`Name: "photoSource", Label: "Photo source", Kind: "select", Options: attachments.SourceOptions()`) &&
    photo.includes(`"photoSource": ""`),
);
pins(
  "photo node declares path/base64/bytes pins",
  photo.includes(`tg.Text("photoPath", "Photo path", domain.PinInput, false)`) &&
    photo.includes(`tg.Text("photoBase64", "Photo base64", domain.PinInput, false)`) &&
    photo.includes(`ID: "photoData"`) &&
    photo.includes(`tg.Text("photoName", "Photo name", domain.PinInput, false)`),
);
pins("photo node registers a resolver", photo.includes("Resolver: resolve, Executor: execute"));

const document = go("internal/nodes/telegram/senddocument/senddocument.go");
pins(
  "document node declares the documentSource select and base64 pin",
  document.includes(`Name: "documentSource", Label: "Document source", Kind: "select", Options: attachments.SourceOptions()`) &&
    document.includes(`tg.Text("documentBase64", "Document base64", domain.PinInput, false)`),
);

const sources = go("internal/nodes/attachments/source.go");
pins(
  "shared source helpers exist with the four modes and Auto option",
  sources.includes(`{Value: "", Label: "Auto — use whatever is set"}`) &&
    sources.includes(`{Value: SourceBytes, Label: "Bytes from another node"}`),
);

const telegramService = go("internal/telegram/actions.go");
pins(
  "telegram service uploads photos as multipart when data is present",
  telegramService.includes(`s.callMultipart(ctx, token, "sendPhoto", photoFormFields(request), formFile{`) &&
    telegramService.includes(`name = "photo.jpg"`),
);

const domain = go("internal/domain/types.go");
pins(
  "TelegramPhotoRequest carries upload payload fields",
  domain.includes("type TelegramPhotoRequest struct") &&
    /Data\s+\[\]byte `json:"-"`/.test(domain.slice(domain.indexOf("type TelegramPhotoRequest struct"))),
);
