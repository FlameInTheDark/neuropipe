# Chat nodes

These nodes operate only in an execution started by a Chat Trigger. They make pipeline conversations responsive without treating chat state as global pipeline state.

## Read Chat History

**Purpose:** retrieve earlier conversation turns. **Pins:** Chat ID and optional Limit inputs; Messages list output. **Configure:** no fields. **Produces:** ordered typed message objects. **Capabilities:** local chat storage only. **Failure:** an invalid chat ID returns no history. **Example:** Chat Trigger Chat ID → Read Chat History → LLM Prompt.

## Reply to Chat

**Purpose:** append an ordered Markdown reply. **Pins:** Exec in/out, Text input, Chat Run ID input. **Configure:** none. **Produces:** the next control pulse. **Capabilities:** local chat storage only. **Failure:** unknown or completed chat runs reject a reply. **Example:** LLM Prompt result → Reply to Chat Text.

## Update Chat Status

**Purpose:** change the active conversation spinner text. **Pins:** Exec in/out, Status input, Chat Run ID input. **Configure:** none. **Produces:** the next control pulse. **Capabilities:** local chat storage only. **Failure:** unknown chat run IDs are reported. **Example:** Chat Trigger → Update Chat Status `Searching…` → HTTP Request.
