# Send Telegram Message

Send one text message to a numeric chat ID or @channel username through the
selected bot identity. Choose Plain text, HTML, or MarkdownV2 parsing; HTML
and MarkdownV2 escape rules belong to Telegram. The node requires the network
capability and returns Sent or Rejected, Message ID, and a rejection Reason
carrying Telegram's own description (for example "Bad Request: chat not
found").

Reply to message ID accepts the numeric Telegram message ID and is sent to
the Bot API as an integer; a non-numeric value is rejected with a precise
reason before any request is made. Numeric pin values wired from
number-producing nodes are converted to ID strings automatically.

Messages longer than Telegram's 4,096-character limit are rejected before any
request is made. Silent suppresses the recipient's notification.
