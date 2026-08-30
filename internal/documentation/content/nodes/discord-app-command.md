# Discord Command Trigger

Start a trusted pipeline when a member runs one of the bot's application
commands — slash commands, user commands, and message commands alike. Unlike
the generic Discord Event Trigger, this node is dedicated to application
commands: it is bound to one registered command, and it exposes the command's
options as individual typed output pins so a pipeline never parses the raw
interaction payload.

Select the bot identity, then pick the command from the Command dropdown; it
lists every command registered on that bot (create and edit commands on the
Discord integration page). Picking a command stores its option schema on the
node, and the canvas immediately grows one output pin per option: text and
snowflake options become Text pins, integer options become Number pins,
number options become Float pins, and boolean options become Boolean pins.
Subcommand groups and subcommands flatten into their value options, matching
the Options map. The full set of user inputs is also available as the Options
map pin.

Beyond the option pins the trigger exposes Command name, Command ID, the
invoking User ID, Username, and Nickname, Channel and Guild IDs, the user's
Locale, and — for user and message context-menu commands — Target user ID,
Target username, and Target message ID. The typed Command record documents
every field, and the Interaction output is the handoff object the reply nodes
need: it carries the interaction id, application id, and token.

The Response mode controls how Discord's response deadlines apply. **Auto
defer** (the default) acknowledges the command the moment it matches a
trusted trigger, showing a loading state to the member and giving the
pipeline the interaction's full 15-minute window to reply — pipelines that
call an LLM or fetch remote data should always defer. **Reply within 3 s**
keeps the initial callback for the Reply to Command node; the reply must then
happen within three seconds of the command, but it can be ephemeral (visible
only to the invoking member). Guild ID and Channel ID conditions filter
deliveries before the pipeline starts; a non-matching command stops the flow
without an error.

Application commands are delivered without any gateway intents, so this
trigger needs no Privileged Gateway Intents toggles. Like every Discord
trigger it only runs when its published pipeline revision is trusted and
enabled.

Wire the Interaction output into Reply to Command, Followup Command Message,
or Edit Command Reply to answer the member.
