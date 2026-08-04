# Local Webhook

The Local Webhook event starts a trusted published pipeline after Neuropipe
receives a correctly HMAC-signed HTTP request. It is intended for local
services, reverse proxies, source-control events, monitoring systems, and
other senders that can sign a request body.

The API listener must be enabled in **Settings → API & Webhooks**. This node
does not listen by itself, and its route is unavailable while the API is off.

## Pins

- **Start** is the event's exec output. Connect it to the first impure action
  or flow-control node.
- The data output carries the received request object. Resolve it with
  **Get Field** to create typed values such as json.event, json.repository,
  or body.

## Configuration

| Field | Required | Guidance |
| --- | --- | --- |
| **Path** | Yes | Use a unique route such as /build-complete. Leading and trailing slashes are normalized. |
| **Signing secret** | Yes | A secret selected from Neuropipe's Windows-protected vault. It is never exposed to the canvas or a run log. |

The delivery URL is:

~~~text
POST http://<configured-bind-address>:<configured-port>/hooks/<path>
~~~

For example, a local listener on the default port with the path
/build-complete receives:

~~~text
POST http://127.0.0.1:7878/hooks/build-complete
~~~

## Authenticate a delivery

Set the X-Neuropipe-Signature request header to:

~~~text
sha256=<hexadecimal HMAC-SHA-256 of the raw body>
~~~

Neuropipe calculates HMAC-SHA-256 using the selected signing secret and
compares it to the header with a timing-safe comparison. It signs the exact raw
body bytes, before JSON parsing. A JSON formatter, newline conversion, or
different encoding between signature calculation and transmission invalidates
the request.

Webhook routes use this HMAC instead of the API's bearer token. The normal /v1
API still follows the authentication mode configured in Settings.

## Produced values

When the signature is valid, the trigger forwards an object containing:

- trigger: the text value webhook;
- body: the raw request body as text;
- json: the parsed body when it contains valid JSON.

For a JSON body of:

~~~json
{"event":"build.completed","build":42}
~~~

configure **Get Field** with the webhook value as its source and add:

- event → json.event as **Text**;
- build → json.build as **Number**.

This makes the graph self-documenting and avoids parsing the same request in
multiple places.

## Execution and approval

Webhook deliveries are unattended. Neuropipe only queues an enabled, published
webhook binding when that exact published revision has a matching trust grant.
Publishing a changed revision may require new trust before the webhook can run
again.

A successful request returns **202 Accepted** with an execution record. The
pipeline runs through Neuropipe's normal bounded execution queue, so acceptance
does not mean every downstream action has finished. Inspect the execution ID
in the desktop app or API to see the final result.

## Failure notes

- The route is unavailable while the embedded API is disabled.
- An unknown or disabled path is rejected.
- Missing secrets and invalid sha256=<hex> signatures are rejected before a
  pipeline starts.
- Untrusted published revisions are rejected for unattended execution.
- Valid HMAC only authenticates the sender. Downstream failures, such as an LLM
  provider error or denied terminal capability, still fail the execution and
  appear in its redacted log.

## Example: create a build report

~~~text
Local Webhook
  Start ──► Get Field (json.repository, json.status)
                  └──► Branch (status == "success")
                             True ──► Format Text ──► Create Report
~~~

Configure the sender to POST its build-completion JSON to
/hooks/build-complete and sign its raw body with the trigger's secret. The
report workflow receives only valid signed deliveries; failed builds can follow
the Branch's false exec output to a notification or separate report.

For end-to-end setup, signing examples, remote exposure guidance, and
troubleshooting, read [API and webhooks](docs:concepts/api-webhooks).
