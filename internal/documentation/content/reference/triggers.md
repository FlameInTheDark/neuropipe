# Trigger nodes

Trigger nodes are events: they have no exec input and begin a run through **Start**. Their **Payload** is an object output; Chat Trigger also exposes typed chat values. Only published triggers appear outside the editor.

## Button Trigger

**Purpose:** start from the Trigger board or an optional global hotkey. **Pins:** Start exec, Payload object. **Configure:** label and optional hotkey. **Produces:** trigger payload. **Approval:** a normal manual run still evaluates downstream capabilities. **Failure:** unpublished buttons and invalid hotkeys cannot activate. **Example:** Button Trigger → HTTP Request → Create Report.

## Cron Trigger

**Purpose:** start on a five-field schedule. **Pins:** Start, Payload. **Configure:** cron expression and IANA/local timezone. **Produces:** schedule payload. **Approval:** unattended use needs trusted published revision. **Failure:** invalid expressions, disabled schedules, and concurrent run skips are recorded. **Example:** Cron Trigger → Read File → Desktop Notification.

## File Watch

**Purpose:** start when a watched file or folder changes. **Pins:** Start, Payload. **Configure:** an approved path. **Produces:** event metadata. **Approval:** file-watch access is scoped to its configured root. **Failure:** inaccessible paths do not silently broaden the watch. **Example:** File Watch → Read File → Structured Extract.

## Global Hotkey

**Purpose:** start from a keyboard shortcut. **Pins:** Start, Payload. **Configure:** a normalized shortcut. **Produces:** trigger payload. **Approval:** downstream capabilities remain visible before use. **Failure:** duplicate hotkeys are rejected. **Example:** Global Hotkey → Run Terminal Command.

## Local Webhook

**Purpose:** receive an HMAC-signed HTTP request through the optional embedded
API. **Pins:** Start and a request object. **Configure:** a unique path and
vault-backed signing secret. **Produces:** trigger, raw body, and parsed JSON
when the body is valid JSON. **Approval:** API must be enabled and the
published revision must be trusted for unattended execution. **Failure:** bad
HMAC, unknown/disabled paths, unavailable API, or untrusted revisions do not
run. **Example:** Local Webhook → Get Field → Create Report. Read the detailed
[Local Webhook node reference](docs:node:trigger:webhook) for raw-body signing,
payload fields, responses, and troubleshooting.

## Chat Trigger

**Purpose:** start a published pipeline from a local conversation. **Pins:** Start exec; Text, Chat ID, and Chat Run ID data outputs. **Configure:** visible chat label. **Produces:** the submitted text and identifiers. **Approval:** pipeline trust applies to its actions. **Failure:** unpublished/inactive bindings are hidden from Chat. **Example:** Chat Trigger → Update Chat Status → Reply to Chat.
