# Send Discord Message

Send one explicit message to a channel snowflake with the selected or default
bot identity. The node requires the network capability. It returns Sent or
Rejected, Message ID, and a rejection Reason carrying Discord's own error
text (for example Missing Permissions). Wire a triggering message ID into
Reply to message ID to answer a specific message.

Channel and reply IDs are validated before any request is made: both must be
numeric Discord snowflakes (up to 20 digits), so a value mangled by a numeric
pipeline — scientific notation like `7.9e+16`, or a truncated paste — is
rejected with a precise reason instead of Discord's opaque Invalid Form Body.
Numeric pin values wired from number-producing nodes are converted to ID
strings automatically.

Reply references get one further pre-flight check, snowflake forensics: a
snowflake encodes its entity's creation time, so a reply that decodes to a
moment before the target channel existed — or into the future — is a truncated
or miscopied ID and is rejected with both decoded dates instead of Discord's
Unknown message, for example `reply to message ID "79216925611139072" decodes
to Aug 2015, before channel 565062979255795712 existed (created Apr 2019)`. A
message sent today has a 19-digit ID; wire the trigger's Message ID output
instead of typing one.

When Discord does reject a request, the Reason pin carries the field-level
detail, for example
`Invalid Form Body — message_reference: Unknown message (REPLIES_UNKNOWN_MESSAGE)`:
the reply target does not exist in the target channel, was deleted, or the
ID belongs to a different channel. When the rejected request carried a reply
reference, the reason also names the two real-world causes — the referenced
message may live in another channel or thread, or the ID is wrong — and points
at the reliable wiring: the trigger's Message ID and Channel ID outputs.

Messages longer than Discord's 2,000-character limit are rejected before any
request is made; Neuropipe never splits them silently.

## Embeds

A message can carry up to ten embeds alongside (or instead of) its text. Two
interchangeable sources exist, and a non-empty one wins:

- **Embed editor** — the Embeds field opens a visual editor modeled on
  embed-generator: a left-hand form (author, title, description, color,
  images, footer, timestamp, and up to 25 fields) with a live Discord preview
  on the right. Every text field accepts `{{variable}}` templates; each
  declared variable becomes a typed input pin on the node, so pipeline data
  lands inside embeds without JavaScript. The editor preview resolves
  templates against the variables' sample values.
- **Embeds JSON** — a pin or field carrying canonical Discord embed JSON,
  either one object or an array, exactly as pasted from Discord's
  documentation or Discohook. Use it when an earlier node (LLM, HTTP, JSON
  query) computes the embed shape at runtime; it overrides the editor.

Discord's embed limits are validated locally before any request — title 256,
description 4,096, author name 256, footer 2,048, field name 256 and value
1,024 characters, 25 fields per embed, 10 embeds per message, and 6,000
characters combined — so a violation is rejected with Neuropipe's precise
reason instead of Discord's Invalid Form Body.

## File attachments

A message can attach up to ten files. The **Image source** dropdown selects
where the image or file comes from — URL, local file, base64 text, or bytes
from another node — and the inspector plus the canvas only show the input
that belongs to the selected source:

- **URL** — File URL, downloaded by the node; multiple URLs may be given,
  one per line.
- **Local file** — File path, a local file read before upload; multiple
  paths, one per line.
- **Base64** — File base64, pasted or wired base64 text (a `data:` URL
  prefix is accepted); File name gives the upload its name and extension.
- **Bytes from another node** — File data, raw bytes wired from a pin. The
  data pin matches the Draw Image node's Image output, so a rendered picture
  connects directly; File name gives the upload its name and extension.

The default, **Auto — use whatever is set**, keeps every input visible and
combines every source that is set, exactly how graphs saved before the
dropdown behaved; a value left behind in a hidden field is ignored once an
explicit source is selected. Each file is capped at 25 MB and checked before
any request. Message text becomes optional when embeds or attachments are
present — Discord requires only one of content, embeds, or files.
