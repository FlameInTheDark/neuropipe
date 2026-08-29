# Draw Image

## Purpose
Compose a pixel-perfect image at pipeline runtime from a visual document
designed in the built-in editor: a fixed-resolution canvas with layers,
styled shapes (rectangles, ellipses, lines, stars), text (static or
interpolated from input pins), and pictures loaded from a URL, a local
path, or a pin value. Use it to build rich cards such as weather
forecasts, leaderboards, or Discord embed images.

## Configuration
- **Image document**: opens the full-screen visual editor. Define the
  canvas resolution and background, organize elements on layers, and
  style each element with solid or gradient fills, strokes with dashes,
  opacity, and rotation. Right-click any element on the canvas for a
  context menu with its actions: edit properties, duplicate, hide,
  z-order (bring to front / send to back), rotate by 90°, center on the
  canvas, add a point to a line, or delete. Right-clicking an element on
  a locked or hidden layer offers to unlock or show that layer, and
  right-clicking empty canvas space inserts a new element at that point
  or toggles grid, snapping, and zoom.
- **Input values**: declare named input pins in the editor (text, number,
  boolean, object, or array). Text elements interpolate them with
  `{{pinName}}` placeholders; any element's visibility can be bound to a
  condition on a pin (for example a boolean `isTrue` check or a number
  comparison); array pins can drive per-item repetition with
  `{{item}}`, `{{item.field}}`, and `{{index}}` placeholders.
- **Output path**: optional file to write the rendered image to.
- **Format**: PNG (lossless) or JPEG with a quality setting.

## Outputs
- **Image**: encoded file bytes (wire type bytes).
- **Base64**: the same bytes as a base64 string, ready for webhooks and
  chat attachments.
- **Result**: record with `path`, `width`, `height`, `sizeBytes`,
  `format`, and `warnings` (for example skipped missing image sources).

## Example
`Webhook Trigger → HTTP Request (weather API) → Draw Image (forecast card
with repeated day columns) → Discord Send Message`
