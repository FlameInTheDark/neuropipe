# Send Telegram Photo

Send one photo through the selected bot identity. The **Photo source**
dropdown selects where the picture comes from — URL, local file, base64
text, or bytes from another node — and the inspector plus the canvas only
show the input that belongs to the selected source:

- **URL** — Photo URL; Telegram fetches the image server-side, so the host
  machine never downloads it.
- **Local file** — Photo path, a local file read and uploaded by the node.
- **Base64** — Photo base64, pasted or wired base64 text (a `data:` URL
  prefix is accepted); Photo name gives the upload its name and extension.
- **Bytes from another node** — Photo data, raw bytes wired from a pin. The
  data pin matches the Draw Image node's Image output, so a rendered picture
  connects directly; Photo name gives the upload its name and extension.

The default, **Auto — use whatever is set**, keeps every input visible:
an upload source (path, base64, or bytes) wins over the URL pass-through,
and with nothing set the node reports that a photo URL, file, base64, or
data is required. An explicit source reports a precise reason when its own
input is empty, and a value left behind in a hidden field is ignored.

An optional Caption (up to 1,024 characters, with the same parse modes as
Send Telegram Message) accompanies the photo. Uploads are capped at 10 MB —
Telegram's photo limit — and checked before any request, so an oversized
file is rejected locally with a precise reason instead of a Telegram error
after a wasted transfer. A photo without a name uploads as `photo.jpg`
with an image/jpeg content type. The node returns Sent or Rejected, the new
Message ID, and Telegram's own rejection description on the Reason pin.
