# Chat

Chat has two modes. **Model** sends the conversation to the active provider. **Pipelines** lists published pipelines with a Chat Trigger.

Pipeline conversations create a distinct chat-run ID and an execution ID. The Chat Trigger provides the user text, chat ID, and chat-run ID. Use **Update Chat Status** for an informative spinner label and **Reply to Chat** for ordered Markdown responses.

Model chats can use Neuropipe tools to list or run published pipelines and inspect reports. State-changing tools either ask for approval or follow the per-conversation allow policy. Tool activity appears as a compact expandable transcript item in the precise historical position where it happened.

Model chats stream assistant answers token by token. Text appears in the reply bubble while the model is still generating (rendered as live Markdown with a caret), and the turn settles into the persisted transcript once it completes. Streaming works with every provider kind — OpenAI-compatible servers, Anthropic, and Ollama's native API — and stopping a run mid-answer closes the live bubble without saving a partial reply.

Use [Read Chat History](docs:reference/chat) to read ordered earlier turns inside a pipeline execution.
