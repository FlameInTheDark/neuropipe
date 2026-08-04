# API and webhooks

Neuropipe's HTTP API is optional and local-first. It is **off by default**:
until you enable it in **Settings → API & Webhooks**, Neuropipe does not open
an HTTP listener and Local Webhook triggers cannot receive requests.

This page explains the API controls and how to deliver a signed webhook safely.
For the node's pins and graph-facing behavior, see the [Local Webhook node
reference](docs:node:trigger:webhook).

## Before you begin

A working webhook needs all of the following:

1. The embedded API is enabled and listening.
2. A pipeline contains a configured **Local Webhook** trigger.
3. The pipeline has been published.
4. The corresponding webhook trigger is enabled and its published revision is
   trusted for unattended execution.
5. The sender signs the **exact raw request body** with that trigger's signing
   secret.

Neuropipe returns an error instead of running a pipeline when any one of those
requirements is missing. This prevents an HTTP request from silently gaining
access to files, terminals, models, or other sensitive pipeline capabilities.

## Enable the local API

Open **Settings → API & Webhooks** and configure the listener:

- **Enabled** turns on Neuropipe's embedded HTTP server.
- **Bind address** is 127.0.0.1 by default. This limits callers to the same
  computer.
- **Port** is 7878 by default. Choose a free port between 1024 and 65535.
- **Authentication** controls the regular /v1 API. Token authentication is the
  default; generate and store the token in the in-app dialog. The token is held
  in the Windows-protected vault and is never shown in diagnostics.
- **Admin API** is optional and only available when token authentication is
  enabled.
- **CORS origins** are only needed for browser clients that call the regular
  API. Add exact origins, such as https://dashboard.example.test; do not use
  them as a substitute for authentication.

Changing the listener to a non-loopback address requires an explicit
acknowledgement. The embedded server serves HTTP only. If another machine must
reach it, put a TLS-terminating reverse proxy in front of Neuropipe, restrict
the network exposure, and keep webhook secrets private.

Disabling the API stops its listener. Existing webhook nodes and bindings stay
in their pipelines, but their routes are unavailable until the API is enabled
again.

## Create a Local Webhook trigger

1. In a draft, add **Local Webhook** from the Trigger category.
2. Set **Path** to a short, unique route, for example
   /build-complete. Leading and trailing slashes are normalized, so
   build-complete, /build-complete, and /build-complete/ describe the same
   route.
3. Create or select a **Signing secret** in the secret picker. Use a long,
   random value and do not paste it into a report, node label, or execution
   input.
4. Wire **Start** to the first impure node in the workflow. Wire the trigger's
   data output into **Get Field**, **Get Variable**, or another typed data node
   when downstream logic needs request values.
5. Save, publish, then enable and trust the resulting trigger binding.

Webhook paths are compared after normalization. Give each live webhook a unique
route; reusing a route makes delivery ownership unclear.

## Endpoint and delivery result

Send a POST request to:

~~~text
http://<bind-address>:<port>/hooks/<path-without-leading-slash>
~~~

For the default local configuration and a path of /build-complete, that is:

~~~text
http://127.0.0.1:7878/hooks/build-complete
~~~

A valid delivery is queued; it does not wait for the pipeline to finish.
Neuropipe responds with **202 Accepted** and an execution record. Use the
execution ID with the regular API or the desktop execution log to inspect the
eventual result.

Webhook routes use their own trigger-specific HMAC verification. They do not
use the regular API bearer token. The /v1 endpoints remain governed by the
authentication mode selected in Settings.

## Sign the raw body

Neuropipe expects this header:

~~~text
X-Neuropipe-Signature: sha256=<lowercase hexadecimal HMAC-SHA-256>
~~~

The HMAC key is the trigger's signing secret. The message is the exact byte
sequence sent as the request body—not a parsed object, a reformatted JSON
string, or a re-serialized payload. JSON whitespace, line endings, and
character encoding therefore matter.

### PowerShell example

The following example creates the body once, calculates the HMAC over its UTF-8
bytes, and sends those same bytes.

~~~powershell
$body = '{"event":"build.completed","build":42}'
$secret = $env:NEUROPIPE_WEBHOOK_SECRET

$bodyBytes = [Text.Encoding]::UTF8.GetBytes($body)
$keyBytes = [Text.Encoding]::UTF8.GetBytes($secret)
$hmac = [Security.Cryptography.HMACSHA256]::new($keyBytes)
$hex = (($hmac.ComputeHash($bodyBytes) | ForEach-Object {
  $_.ToString("x2")
}) -join "")

Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:7878/hooks/build-complete" -ContentType "application/json" -Headers @{ "X-Neuropipe-Signature" = "sha256=$hex" } -Body $body
~~~

Keep the secret in an environment variable, a secret manager, or the sending
system's credential store. Do not hard-code it in a source file.

### curl example

Calculate the signature with a tool that receives the same body bytes that curl
sends. For a small test payload:

~~~bash
body='{"event":"build.completed","build":42}'
signature=$(printf %s "$body" | openssl dgst -sha256 -hmac "$NEUROPIPE_WEBHOOK_SECRET" -hex | sed 's/^.* //')

curl --request POST "http://127.0.0.1:7878/hooks/build-complete" \
  --header "Content-Type: application/json" \
  --header "X-Neuropipe-Signature: sha256=$signature" \
  --data-binary "$body"
~~~

Use --data-binary for signed payloads. A client that rewrites newlines, changes
encoding, or adds a trailing newline after calculating the signature will fail
validation.

## Read the webhook payload in a Blueprint

The Local Webhook event forwards an object to its output. It always includes:

| Field | Type | Meaning |
| --- | --- | --- |
| trigger | text | The value webhook. |
| body | text | The received raw request body as text. |
| json | any, optional | Parsed JSON when the body is valid JSON. |

Use **Get Field** to make the values you need explicit and typed. For example,
set its source to the trigger output and configure these outputs:

| Output pin | Field path | Type |
| --- | --- | --- |
| event | json.event | Text |
| build | json.build | Number |

Then connect the typed pins to a **Branch**, **Format Text**, **Create Report**,
or other node. A non-JSON body is still valid if its signature is correct; in
that case use body or parse it deliberately with the appropriate data node.

## Trust, capabilities, and queueing

A webhook is unattended input. Neuropipe therefore uses the published revision
and applies its revision-scoped trust rule before it queues the execution.
Editing and publishing a changed revision requires you to review and trust that
new revision before it can receive unattended webhook work.

The execution joins Neuropipe's owned bounded queue. A 202 response means it
was accepted for queueing, not that downstream HTTP calls, LLM requests,
terminal commands, or reports have completed successfully. Check the execution
log for the final status. Queue and execution errors are recorded with normal
redaction; signing secrets and payloads are not exposed through diagnostics.

## Security checklist

- Keep the listener on 127.0.0.1 unless remote access is genuinely required.
- Give every source system its own random signing secret and route.
- Verify the HMAC over raw bytes before processing payloads; Neuropipe does
  this with a timing-safe comparison.
- Use HTTPS via a reverse proxy for any non-loopback deployment.
- Limit the reverse proxy to expected source networks where possible.
- Keep webhook workflows small and grant only the capabilities they require.
- Review and re-trust a pipeline after publishing capability changes.
- Rotate a signing secret by updating the vault value or selecting a replacement,
  then update the sender before disabling the old value.

## Troubleshooting

| Symptom | Likely cause | What to check |
| --- | --- | --- |
| Connection refused | The API is off, on another port, or bound to another address. | Enable the API and confirm the displayed endpoint in Settings. |
| 404 route not found | The path does not match an enabled webhook trigger. | Check normalized path spelling and ensure the trigger is published. |
| Signature invalid | The secret, header format, or signed bytes differ. | Send X-Neuropipe-Signature: sha256=<hex> and sign the exact --data-binary/body bytes. |
| Approval or trust error | The published revision is not trusted for unattended runs. | Review the revision's capability grant and enable/trust the trigger binding. |
| 202 but no visible effect | The run is queued or a downstream node failed. | Open the returned execution ID in the execution log and inspect the first failed node. |
| JSON fields are empty | The body was not valid JSON or the field path is wrong. | Read body, inspect json, and use paths such as json.event. |

## Related reading

- [Local Webhook node reference](docs:node:trigger:webhook)
- [Publishing and trust](docs:concepts/publishing-trust)
- [Runs and queues](docs:concepts/runs-and-queues)
- [Blueprint exec and data pins](docs:concepts/blueprint-exec-data)
