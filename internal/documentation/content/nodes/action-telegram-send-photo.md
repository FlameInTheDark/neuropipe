# Send Telegram Photo

Send one photo by URL; Telegram fetches the image server-side, so the host
machine never downloads it. An optional Caption (with the same parse modes as
Send Telegram Message) accompanies the photo. The node returns Sent or
Rejected and the new Message ID; invalid URLs surface as a soft rejection
with Telegram's description.
