# Send Twitch Chat Message

Send one explicit Text message to a channel name (for example, `your_channel`)
with the selected or default bot
identity. The node requires the network capability. It returns Sent or Rejected,
Message ID, and a rejection Reason. Connect a triggering chat Message ID to
Reply to message ID to create a reply.

Messages longer than Twitch’s 500-character limit are rejected; Neuropipe never
splits them or impersonates the incoming author.
