# Twitch Event Trigger

Starts a trusted, enabled published pipeline from a Twitch EventSub delivery.
Choose the EventSub event, authorization identity, and channel name (its Twitch
login, not its numeric user ID). Chat events
also expose typed message text, command text, broadcaster, author, and message
ID. Prefix and author allow-list filters both apply when configured.

The EventSub connection deduplicates delivery IDs and fans one upstream
subscription out to all matching trusted bindings.
