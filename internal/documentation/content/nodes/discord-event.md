# Discord Event Trigger

Start a trusted pipeline from one Discord gateway event. Add it from the
Discord category of the node library, pick the bot identity and the event; the
gateway service computes the required intents from trusted, enabled triggers
only, so a pasted pipeline never silently opens a privileged event stream.

The configuration adapts to the selected event. Every event offers the Guild
ID and (where applicable) Channel ID conditions, which filter deliveries
before the pipeline starts. Message events additionally expose the Message
prefix, Author IDs, and Case-sensitive prefix filters; Interaction events
offer a Command name filter; reaction events offer a Reaction emoji filter
(unicode or `name:id` form); and member events offer a User IDs filter. A
non-matching event stops the flow without an error.

Message events expose Text, Command text (the message after the optional
Message prefix), Channel and Guild IDs and names, and a typed Author record.
Interaction events expose Command name, flattened Options, and the invoking
user. Reaction events expose Emoji and User ID, member events expose User ID,
Username, Nickname, and Joined at, ban events expose User ID and Username, and
voice state events expose User ID, Channel ID, and Session ID.

Every object pin documents its payload structure: hover the Event or Author pin
(the label or the dot) and the tooltip lists the record name (for example
`DiscordEvent` or `DiscordAuthor`) together with each field and its type — so
you can wire `Get field` nodes to `event.messageId` or `author.username`
without guessing.

Listening for *Message Content* and *Server Members* events requires the
matching Privileged Gateway Intents toggles in the Discord Developer Portal;
the Discord settings page surfaces a warning until they are enabled.
