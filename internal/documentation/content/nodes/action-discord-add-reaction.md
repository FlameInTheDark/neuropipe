# Add Discord Reaction

React to one message with an emoji. Pass a unicode emoji (for example `👋`) or
a custom emoji in `name:id` form. The node requires the network capability and
returns Done or Rejected with Discord's rejection Reason — most commonly
Missing Permissions when the bot lacks the Add Reactions permission.
