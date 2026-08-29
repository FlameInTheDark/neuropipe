# Send Telegram Document

Send one general file with an optional caption — Telegram's "message with
file", because the Bot API cannot send text and a file in one call: the
caption (up to 1,024 characters, with the same parse modes as Send Telegram
Message) is the message text.

The **Document source** dropdown selects where the file comes from — URL,
local file, base64 text, or bytes from another node — and the inspector plus
the canvas only show the input that belongs to the selected source:

- **URL** — Document URL; Telegram fetches the file server-side, exactly
  like the Send Telegram Photo node.
- **Local file** — Document path, a local file read by the node before
  upload, for reports written by Write File or Draw Image's output path.
- **Base64** — Document base64, pasted or wired base64 text (a `data:` URL
  prefix is accepted); File name gives the upload its name and extension.
- **Bytes from another node** — Document data, raw bytes wired from a pin.
  The data pin matches the Draw Image node's Image output, so a rendered
  picture connects directly; File name gives the upload its name and
  extension.

The default, **Auto — use whatever is set**, keeps every input visible and
keeps the pre-dropdown behaviour: a path or data pin uploads as a file and
wins over the URL pass-through; with nothing set the node reports that a
document URL, path, base64, or data is required. An explicit source reports
a precise reason when its own input is empty, and a value left behind in a
hidden field is ignored.

Uploads are capped at 50 MB and checked before any request, so an oversized
file is rejected locally with a precise reason instead of a Telegram error
after a wasted transfer. The node returns Sent or Rejected, the new Message
ID, and Telegram's own rejection description on the Reason pin.
