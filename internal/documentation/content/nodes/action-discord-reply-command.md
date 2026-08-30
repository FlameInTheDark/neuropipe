# Reply to Command

Answer the application command that started the pipeline. Wire the Discord
Command Trigger's **Interaction** output into this node's Interaction pin,
then provide the reply as message text, embeds, or both — the embed editor
and Embeds JSON pin work exactly like on Send Discord Message.

The node adapts to how the trigger responded to Discord. When the trigger
uses **Auto defer** (the default), the member saw a loading state and this
node replaces it with the reply, so the pipeline can take its time — up to
the interaction token's 15-minute lifetime. When the trigger uses **Reply
within 3 s**, this node sends the initial response itself, which only works
while the command is still fresh; a late reply is rejected with Discord's
own error message.

The Ephemeral toggle makes the reply visible only to the member who ran the
command. Ephemeral replies require the trigger's manual response mode,
because a deferred response is already public; enabling the toggle with
Auto defer is rejected with an explanation rather than silently sending a
public message.

Message text supports Discord embeds through the Embeds editor: declare
`{{template}}` variables, wire typed data into the generated pins, and the
reply renders with live values. A non-empty Embeds JSON pin overrides the
editor document, so a pipeline can compute embed JSON dynamically.

Ports: **Sent** when Discord accepted the reply (carrying the reply's Message
ID), **Rejected** with a Reason output when the interaction expired, the
token was invalid, or the body failed validation.

To send additional messages afterwards, use Followup Command Message; to
change this reply later, use Edit Command Reply.
