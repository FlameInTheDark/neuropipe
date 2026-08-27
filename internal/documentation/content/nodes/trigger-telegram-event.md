# Telegram Event Trigger

Start a trusted pipeline from one Telegram bot update. Pick the bot identity
and the update type; the polling service recomputes `allowed_updates` from
trusted, enabled triggers only, so an untrusted binding never receives
events.

Message-like updates (private chats, groups, channel posts) expose Text,
Command text (the message after the optional Message prefix), Chat ID, Chat
type and title, and a typed From record. Callback queries expose Callback
data, Callback query ID, and the originating chat and message. The optional
Chat IDs allowlist (numeric IDs or @channel usernames) filters deliveries
before the pipeline starts; prefix and From usernames filters run inside the
node.

Group privacy mode limits what bots see in groups; see the Telegram guide for
the two supported configurations.
