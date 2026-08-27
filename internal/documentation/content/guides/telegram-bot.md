# Telegram bots

## Create the bot

Talk to [@BotFather](https://t.me/botfather) on Telegram, send
`/newbot`, and follow the prompts. BotFather prints the bot token —
Neuropipe needs only this token. Static tokens do not expire, but they can be
regenerated with `/revoke`; the hourly validation marks such identities
invalid.

## Chat IDs

Telegram addresses chats by numeric ID: positive for private chats, negative
for groups, and `-100…` for supergroups and channels. The simplest way to
find one is a temporary `message` trigger whose Chat ID output you inspect in
a run record. Public channels also accept their `@username`.

## Privacy mode — reading group messages

By default, bots in groups only receive commands and mentions. For full
message triggers in groups either disable privacy mode with
`/setprivacy` → *Disable* (bot must then be re-added to the group) or make
the bot a group administrator. Private chats and channels (where the bot is
admin) always deliver every message.

## Connect the identity

In **Settings → Telegram**, choose **Add bot** and paste the token. Neuropipe
validates it with `getMe`, stores it only in the Windows-protected local
vault, and lists the resolved bot identity. One bot token permits exactly one
polling consumer: a second client on the same token (a phone session running
the bot, another Neuropipe instance) produces a 409 conflict surfaced on the
settings page.

## Telegram Event Trigger

Add **Telegram Event Trigger** from the Telegram category of the node library,
then choose an update type in the inspector. The long-polling service
recomputes `allowed_updates` from **trusted, enabled** triggers only, so an
untrusted binding never receives events. Message-like updates expose Text,
Command text (the message minus the configured prefix), Chat ID, Chat type
and title, and a typed From record. Callback queries expose Callback data and
Callback query ID for **Answer Telegram Callback**. Bot member updates expose
the Old and New status transitions, and join requests and reactions expose
the acting user.

Updates that arrived while Neuropipe was closed are deliberately discarded:
polling starts from the newest update, mirroring Twitch's live-only delivery.
The filter set adapts to the selected update. The optional Chat IDs allowlist
(numeric IDs or @channel usernames) filters deliveries before the pipeline
starts. Message-like updates add Prefix, From usernames, and Case-sensitive
prefix filters; callback queries add a Callback data prefix filter for
routing inline keyboard buttons. A non-matching update simply stops the flow.

Example: configure `message`, prefix `/alert`. Connect Text to a formatter
and Chat ID to **Send Telegram Message**. Publish the pipeline, trust the
trigger on the settings page, then send `/alert hello` to the bot.

## Sending messages

**Send Telegram Message** supports Plain text, HTML, and MarkdownV2 parse
modes and returns Sent or Rejected with Telegram's own description (for
example "Bad Request: chat not found"). **Send Telegram Photo** takes a URL
Telegram fetches server-side. **Edit**, **Delete**, **Pin**, **Chat Action**,
and **Answer Callback** complete the moderation surface. All sends are
pre-validated against Telegram's 4,096-character limit; group chats are
rate-limited to about one message per second by Telegram itself.
