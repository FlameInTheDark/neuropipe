# Send Discord Message

Send one explicit message to a channel snowflake with the selected or default
bot identity. The node requires the network capability. It returns Sent or
Rejected, Message ID, and a rejection Reason carrying Discord's own error
text (for example Missing Permissions). Wire a triggering message ID into
Reply to message ID to answer a specific message.

Channel and reply IDs are validated before any request is made: both must be
numeric Discord snowflakes (up to 20 digits), so a value mangled by a numeric
pipeline — scientific notation like `7.9e+16`, or a truncated paste — is
rejected with a precise reason instead of Discord's opaque Invalid Form Body.
Numeric pin values wired from number-producing nodes are converted to ID
strings automatically.

Reply references get one further pre-flight check, snowflake forensics: a
snowflake encodes its entity's creation time, so a reply that decodes to a
moment before the target channel existed — or into the future — is a truncated
or miscopied ID and is rejected with both decoded dates instead of Discord's
Unknown message, for example `reply to message ID "79216925611139072" decodes
to Aug 2015, before channel 565062979255795712 existed (created Apr 2019)`. A
message sent today has a 19-digit ID; wire the trigger's Message ID output
instead of typing one.

When Discord does reject a request, the Reason pin carries the field-level
detail, for example
`Invalid Form Body — message_reference: Unknown message (REPLIES_UNKNOWN_MESSAGE)`:
the reply target does not exist in the target channel, was deleted, or the
ID belongs to a different channel. When the rejected request carried a reply
reference, the reason also names the two real-world causes — the referenced
message may live in another channel or thread, or the ID is wrong — and points
at the reliable wiring: the trigger's Message ID and Channel ID outputs.

Messages longer than Discord's 2,000-character limit are rejected before any
request is made; Neuropipe never splits them silently.
