# Twitch Event Trigger

Starts a trusted, enabled published pipeline from a Twitch EventSub delivery.
Choose the EventSub event, authorization identity, and channel name (its Twitch
login, not its numeric user ID). Chat events
also expose typed message text, command text, broadcaster, author, and message
ID. Prefix and author allow-list filters both apply when configured.

Hovering an object pin (the label or the dot) shows its record structure: the
tooltip lists each field of the Event envelope or the Author record with its
type, so downstream `Get field` nodes can be wired without guessing.

The EventSub connection deduplicates delivery IDs and fans one upstream
subscription out to all matching trusted bindings.
