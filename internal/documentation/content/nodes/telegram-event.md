# Telegram Event Trigger

Start a trusted pipeline from one Telegram bot update. Add it from the
Telegram category of the node library, pick the bot identity and the update
type; the polling service recomputes `allowed_updates` from trusted, enabled
triggers only, so an untrusted binding never receives events.

The configuration adapts to the selected update. Every event offers the
Chat IDs allowlist (numeric IDs or @channel usernames), which filters
deliveries before the pipeline starts. Message-like updates (private chats,
groups, channel posts) additionally expose the Message prefix, From
usernames, and Case-sensitive prefix filters, which run inside the node.
Callback queries offer a Callback data prefix filter for routing inline
keyboard buttons. A non-matching update stops the flow without an error.

Message-like updates expose Text, Command text (the message after the
optional Message prefix), Chat ID, Chat type and title, and a typed From
record. Callback queries expose Callback data, Callback query ID, and the
originating chat and message. Bot member updates expose the Old and New
status transitions, join requests expose the requesting user, and reaction
updates expose the reacting user and message.

Every object pin documents its payload structure: hover the Event or From pin
(the label or the dot) and the tooltip lists the record name (for example
`TelegramEvent` or `TelegramFrom`) together with each field and its type — so
you can wire `Get field` nodes to `event.updateId` or `from.username` without
guessing.

Group privacy mode limits what bots see in groups; see the Telegram guide for
the two supported configurations.
