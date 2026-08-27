# Send Discord Direct Message

Send one direct message to a user snowflake through the selected bot identity.
The node requires the network capability and returns Sent or Rejected,
Message ID, and a rejection Reason. Discord refuses DMs when the user shares
no server with the bot or disabled direct messages; that refusal arrives as a
soft Rejected result with the API's description, not a node error.

Messages longer than Discord's 2,000-character limit are rejected before any
request is made.
