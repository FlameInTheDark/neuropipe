# Twitch EventSub

## Connect an identity

In **Integrations → Twitch**, enter the public Client ID from your Twitch
application, then choose **Connect Twitch**. The device-code dialog shows the
verification code and opens Twitch’s verification page. Access and refresh
tokens are stored only in the Windows-protected local vault and never return to
the renderer. A manually supplied token is an advanced fallback: it is
validated before storage and may require reconnection because it cannot always
be refreshed.

Each trigger selects both its channel name and authorization identity. Enter
the channel login (for example, `your_channel`), not a numeric Twitch user ID;
Neuropipe resolves the ID locally when it creates the EventSub subscription.
The
EventSub service reconnects, coalesces equal upstream subscriptions, deduplicates
delivery IDs, and queues only enabled, trusted published bindings.

## Twitch Event Trigger

Choose an EventSub event in the inspector. Every event includes Event, Event
type, Subscription ID, and Received at. `channel.chat.message` additionally
exposes Text, Command text, the numeric Broadcaster ID, Author ID, Message ID, Author, and
Channel. Message prefix matching is case-insensitive by default and does not
alter raw text; both configured prefix and author allow-list must match.

Example: configure `channel.chat.message`, channel `your_channel`, prefix `!hello`.
Connect Text to a formatter and Message ID to **Send Twitch Chat Message**’s
Reply to message ID. The reply remains attached to the incoming message.

For an author-filtered command, set a prefix such as `!mod` and list the allowed
numeric author IDs in **Author IDs**. Both the prefix and author filter must
match; the original message Text remains unchanged.

## Send Twitch Chat Message

This action needs the network capability, a channel name, and Message. It
uses its selected bot identity or the default bot identity; it never impersonates
the incoming author. Twitch limits one message to 500 characters. Neuropipe
rejects longer messages rather than splitting them. It exposes Sent and
Rejected paths plus Message ID and Reason.

For a channel-point workflow, select
`channel.channel_points_custom_reward_redemption.add`, connect the desired
values to your response logic, then send one chat message. Events whose scopes
are not granted remain visible but cannot activate until the identity reconnects
with those scopes.
