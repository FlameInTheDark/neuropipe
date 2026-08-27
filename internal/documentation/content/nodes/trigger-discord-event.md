# Discord Event Trigger

Start a trusted pipeline from one Discord gateway event. Pick the bot identity
and the event; the gateway service computes the required intents from trusted,
enabled triggers only, so a pasted pipeline never silently opens a privileged
event stream.

Message events expose Text, Command text (the message after the optional
Message prefix), Channel and Guild IDs and names, and a typed Author record.
Interaction events expose Command name, flattened Options, and the invoking
user. The optional Guild ID and Channel ID conditions filter deliveries before
the pipeline starts; prefix and Author IDs filters run inside the node so a
non-matching message stops the flow without an error.

Listening for *Message Content* and *Server Members* events requires the
matching Privileged Gateway Intents toggles in the Discord Developer Portal;
the Discord settings page surfaces a warning until they are enabled.
