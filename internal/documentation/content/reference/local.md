# Local nodes

Local action nodes use scoped capability approval. Configure only roots and repositories you expect the published revision to access.

## List Directory

**Purpose:** list direct files, folders, and symbolic links in one approved directory. **Pins:** Exec input/output, Path input, Files list output. **Produces:** `name`, `path`, `size` in bytes, `type` (`file`, `directory`, or `symlink`), `updatedAt`, and `createdAt` when the platform provides it. **Capability:** file read. **Failure:** nonexistent, unreadable, or unapproved directories fail. **Example:** Button Trigger → List Directory → For Each Loop.

## Read File

**Purpose:** read a local file without changing its bytes. **Pins:** Exec input/output, Path input, one Result output. **Configure:** select Bytes or Text for Result. **Produces:** the selected representation; choosing Text for non-UTF-8 content fails safely. **Capability:** file read. **Failure:** nonexistent or unapproved paths fail. **Example:** File Watch → Read File → Base64 Encode.

Use **Base64 Encode** and **Base64 Decode** to explicitly select text or byte-slice input and output representations. No local node performs this conversion implicitly.

## Write File

**Purpose:** write text or raw bytes. **Pins:** Exec input/output, Path/Content inputs, Result object. **Configure:** path, Content type, and text content when Text is selected. **Produces:** written path and Boolean. Bytes must arrive through a connected Bytes pin; they are never parsed from text. **Capability:** file write. **Failure:** parent access and write errors are logged. **Example:** Read File (Bytes) → Write File (Bytes).

## Run Terminal Command

**Purpose:** execute PowerShell, Windows PowerShell, or cmd. **Pins:** Exec input/output, Shell/Command/Working Directory inputs, Result object. **Configure:** shell and command. **Produces:** command and combined output. **Capability:** terminal. **Failure:** cancellation, non-zero process failures, and invalid workspace access stop the node. **Example:** Button Trigger → Run Terminal Command → Get Field `terminal.output`.

## Desktop Notification

**Purpose:** show a Windows notification. **Pins:** Exec input/output, Title/Message inputs, Result object. **Configure:** title and message. **Produces:** displayed title/message. **Capability:** none. **Failure:** unsupported platform notification failures are logged. **Example:** Branch True → Desktop Notification.

## Git

**Purpose:** run a focused Git operation. **Pins:** Exec input/output, Operation/Repository inputs, Result object. **Configure:** supported operation and repository. **Produces:** operation and output. **Capability:** Git. **Failure:** repository and command failures are recorded. **Example:** Cron Trigger → Git status → Create Report.
