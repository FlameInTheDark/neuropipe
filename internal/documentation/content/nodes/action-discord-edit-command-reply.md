# Edit Command Reply

Change what the bot already replied to an application command. Wire the
Discord Command Trigger's **Interaction** output into the Interaction pin,
then provide the new message as text, embeds, or both.

Leave the Message ID empty to edit the original reply — the message the
member saw as the command's answer. Provide a followup message's ID (for
example the Message ID output of Followup Command Message) to edit that
followup instead. Editing works for 15 minutes after the command ran, the
lifetime of the interaction token; afterwards the edit is rejected with
Discord's own error.

The embed editor and Embeds JSON pin behave like on Send Discord Message:
declare `{{template}}` variables, wire typed data into the generated pins,
and a non-empty Embeds JSON pin overrides the editor document. Edits replace
the previous body entirely, so include everything the message should show
afterwards.

Ports: **Done** when Discord accepted the edit, **Rejected** with a Reason
output when the window has passed, the message id is invalid, or the body
failed validation.
