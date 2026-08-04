# Local nodes

Local action nodes use scoped capability approval. Configure only roots and repositories you expect the published revision to access.

## Read File

**Purpose:** read text, JSON, or CSV. **Pins:** Exec input/output, Path input, Result object. **Configure:** path. **Produces:** file path, content, and parsed JSON when available. **Capability:** file read. **Failure:** nonexistent or unapproved paths fail. **Example:** File Watch → Read File → Parse JSON.

## Write File

**Purpose:** write text or JSON. **Pins:** Exec input/output, Path/Content inputs, Result object. **Configure:** path and content. **Produces:** written path and Boolean. **Capability:** file write. **Failure:** parent access and write errors are logged. **Example:** LLM Prompt → Write File.

## Run Terminal Command

**Purpose:** execute PowerShell, Windows PowerShell, or cmd. **Pins:** Exec input/output, Shell/Command/Working Directory inputs, Result object. **Configure:** shell and command. **Produces:** command and combined output. **Capability:** terminal. **Failure:** cancellation, non-zero process failures, and invalid workspace access stop the node. **Example:** Button Trigger → Run Terminal Command → Get Field `terminal.output`.

## Desktop Notification

**Purpose:** show a Windows notification. **Pins:** Exec input/output, Title/Message inputs, Result object. **Configure:** title and message. **Produces:** displayed title/message. **Capability:** none. **Failure:** unsupported platform notification failures are logged. **Example:** Branch True → Desktop Notification.

## Git

**Purpose:** run a focused Git operation. **Pins:** Exec input/output, Operation/Repository inputs, Result object. **Configure:** supported operation and repository. **Produces:** operation and output. **Capability:** Git. **Failure:** repository and command failures are recorded. **Example:** Cron Trigger → Git status → Create Report.
