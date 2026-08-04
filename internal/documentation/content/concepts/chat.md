# Chat

Chat has two modes. **Model** sends the conversation to the active provider. **Pipelines** lists published pipelines with a Chat Trigger.

Pipeline conversations create a distinct chat-run ID and an execution ID. The Chat Trigger provides the user text, chat ID, and chat-run ID. Use **Update Chat Status** for an informative spinner label and **Reply to Chat** for ordered Markdown responses.

Model chats can use Neuropipe tools to list or run published pipelines and inspect reports. State-changing tools either ask for approval or follow the per-conversation allow policy. Tool activity appears as a compact expandable transcript item in the precise historical position where it happened.

Use [Read Chat History](docs:reference/chat) to read ordered earlier turns inside a pipeline execution.
