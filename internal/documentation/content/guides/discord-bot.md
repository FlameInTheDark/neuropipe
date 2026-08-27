# Discord bots

## Create the bot

Open the [Discord Developer Portal](https://discord.com/developers/applications)
and create an application. Under **Bot**, choose **Reset Token** and copy the
bot token — Neuropipe needs only this token, no client secret or OAuth flow.
Enable the **Privileged Gateway Intents** your automations need:

- **Message Content Intent** — required for message text triggers.
- **Server Members Intent** — required for member join/leave/update triggers.

Unverified personal bots (up to 100 servers) can enable both toggles
immediately. Under **Installation**, generate an invite link with the
permissions your pipelines need: *Send Messages*, *Read Message History*,
*Add Reactions*, and *Manage Messages* cover the built-in nodes.

## Connect the identity

In **Settings → Discord**, choose **Add bot** and paste the token. Neuropipe
validates it with a REST `users/@me` call, stores it only in the
Windows-protected local vault, and lists the resolved bot identity. Tokens are
static secrets — they do not expire like OAuth tokens — but they can be
revoked in the portal; the hourly validation marks such identities invalid.

## Discord Event Trigger

Add **Discord Event Trigger** from the Discord category of the node library,
then choose a gateway event in the inspector. The polling-free gateway service
computes the intent union from **trusted, enabled** triggers only, so a pasted
pipeline never silently enables a privileged event stream. Every event
includes the typed Event envelope, Event type, Gateway event, Message ID, and
Received at. Message events additionally expose Text, Command text (the
message minus the configured prefix), Channel and Guild IDs with cached
names, and a typed Author record. Interaction events expose Command name,
flattened Options, and the invoking user. Reaction events expose Emoji and
User ID, member events expose User ID, Username, Nickname, and Joined at, and
voice state events expose User ID, Channel ID, and Session ID.

The filter set adapts to the selected event. The optional Guild ID and Channel
ID conditions are snowflakes; leave them empty to match every server or
channel. Message events add Prefix, Author IDs, and Case-sensitive prefix
filters; interaction events add a Command name filter; reaction events add a
Reaction emoji filter; member events add a User IDs filter. All node-side
filters work the same way: an event that does not match simply stops the flow.

Example: configure `message.create`, prefix `!hello`. Connect Text to a
formatter and the trigger's Channel ID to **Send Discord Message**. Publish
the pipeline, trust the trigger on the settings page, then send `!hello` in a
channel the bot can read.

## Sending messages and reactions

**Send Discord Message** and **Send Discord Direct Message** return Sent or
Rejected with Discord's own rejection reason, most commonly *Missing
Permissions*. **Add Discord Reaction** accepts unicode emoji or `name:id`
custom emoji. **Edit** and **Delete Message** operate on messages the bot
sent or can moderate. All sends are pre-validated against Discord's
2,000-character limit.

To answer a specific message, wire its Message ID into **Send Discord
Message**'s Reply to message ID. Channel and reply IDs are validated as
snowflakes (up to 20 digits) before any request, and numeric pin values from
number-producing nodes are converted automatically. Reply IDs also go through
snowflake forensics: the reply ID and the channel ID both encode their
creation time, so a reply that predates the channel — or is dated in the
future — is rejected with both decoded dates (for example `decodes to Aug
2015, before channel … existed (created Apr 2019)`) instead of Discord's
opaque Unknown message; that shape means the ID was truncated while copying.
When Discord rejects a request, the Reason pin carries the field-level detail
— for example
`Invalid Form Body — message_reference: Unknown message (REPLIES_UNKNOWN_MESSAGE)`
means the reply target does not exist in the target channel (deleted, wrong
channel, or a mangled ID), and the reason appends the same guidance. To reply
to the message that triggered the flow, wire the trigger's Message ID output
rather than typing the ID by hand — hand-copied IDs lose digits easily, and a
current message ID has 19 digits — and wire the trigger's Channel ID output
into the node's Channel ID input as well, so the reply always lands in the
channel the message actually came from.
