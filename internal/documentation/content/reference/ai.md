# AI nodes

AI action nodes use their selected provider/model (defaulting to the app's default provider and its default model) and execute through an exec input. Their resolved prompts and outputs are recorded redacted in the execution log. Set an LLM queue limit appropriate for the active local runtime.

Every AI node can publish live progress to the chat run that triggered it: turn on **Update chat status** to reveal a **Chat Run ID** input pin (wire `Chat Trigger.Chat Run ID`). The node then reports "Thinking" while the model works, and agents additionally report "Running <tool>" for every connected tool call, until the run finishes with its normal final status.

## LLM Prompt

**Purpose:** generate text. **Pins:** Exec input/output, Prompt/Model inputs, Result object. **Configure:** prompt and optional model override. **Produces:** `llm.content` and, when available, `llm.json`. **Capability:** provider/network as configured. **Failure:** unavailable model, provider errors, or cancellation stop the node. **Example:** Get Field → LLM Prompt → Create Report.

## Structured Extract

**Purpose:** ask for schema-shaped JSON. **Pins:** Exec input/output, Instructions input, Result. **Configure:** fields with names, types, descriptions, and required state. **Produces:** `llm.content` and `llm.json`. **Capability:** provider/network. **Failure:** schema-invalid model output is an error. **Example:** Read File → Structured Extract → Get Field.

## LLM Boolean Router

**Purpose:** make a constrained true/false decision. **Pins:** Exec input; True, False, Error exec outputs; Result. **Configure:** question. **Produces:** decision and model content. **Capability:** provider/network. **Failure:** the model must call `route({ decision: "true" | "false" })`; invalid calls take Error. **Example:** Boolean Router True → notification, False → report.

## LLM Choice Router

**Purpose:** choose a stable configured option. **Pins:** Exec input; option exec outputs plus Error; Result. **Configure:** option ID, title, and model-facing description. **Produces:** selected option ID and content. **Capability:** provider/network. **Failure:** changing/removing an option requires reconnecting affected output edges. **Example:** Choice Router `urgent` → Run Pipeline.

## Summarize

**Purpose:** create a concise summary. **Pins:** Exec input/output, Instructions input, Result. **Configure:** instructions. **Produces:** `llm.content`. **Capability:** provider/network. **Failure:** provider issues appear in the run log. **Example:** HTTP Result → Summarize → Create Report.

## Agent

**Purpose:** solve a task with explicitly connected published LLM tool functions. **Pins:** Exec input, an unlimited **Tools** tool input, Exec output, and Result. **Configure:** instructions, mode, maximum turns, and unlimited turns. **Modes:** **One message** (default) sends the instructions plus connected input as a single user turn; **Chat history** reveals a **Chat ID** input pin, loads that conversation's earlier user and assistant turns, and answers the latest user message directly — wire `Chat Trigger.Chat ID` to run the agent as a chat agent with tools. Instructions stay available as the system prompt in both modes. **Turns:** the budget caps tool rounds (default 8, at most 32); **Unlimited turns** removes the cap for long-running tasks, leaving cancellation as the only stop condition. **Produces:** content and typed tool results for subsequent model turns. **Capability:** connected tool capabilities remain subject to normal publication trust. **Failure:** turn budgets stop safely; invalid arguments return safe contract feedback to the model. **Example:** `Get city forecast.Tool → Agent.Tools`; Agent → Report. See **LLM tool functions** for the complete contract and examples.

## Coding Agent

**Purpose:** run a bounded coding task in a scoped workspace. **Pins:** Exec input/output and Result. **Configure:** task, workspace, mode, maximum turns, and unlimited turns. **Produces:** model result and tool activity. **Capabilities:** file read/write, terminal, and Git. **Failure:** it always needs an approval/trust scope; cancellation stops managed work. **Modes:** the same **One message** / **Chat history** switch and **Chat ID** pin as the Agent, with the coding task as the system prompt. **Example:** Chat Trigger → Coding Agent → Reply to Chat.
