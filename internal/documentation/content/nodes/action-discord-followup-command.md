# Followup Command Message

Send an additional message after the command has been answered. While an
interaction token is valid — 15 minutes after the member ran the command —
Discord lets the bot send followup messages through it; this node sends one
such message.

Wire the Discord Command Trigger's **Interaction** output into the
Interaction pin, then provide the message as text, embeds, or both. The
embed editor and Embeds JSON pin behave like on Send Discord Message:
declare `{{template}}` variables, wire typed data into the generated pins,
and a non-empty Embeds JSON pin overrides the editor document.

Followups need the trigger's **Auto defer** response mode (or an already
answered interaction): a manual interaction that was never answered is
rejected with an explanation instead of failing against Discord's endpoint.
Replies sent this way appear as normal bot messages in the channel; they
cannot be ephemeral.

Ports: **Sent** when Discord accepted the message — the Message ID output
carries the new message's id, which Edit Command Reply can later change —
and **Rejected** with a Reason output when the 15-minute window has passed,
the token is invalid, or the body failed validation.
