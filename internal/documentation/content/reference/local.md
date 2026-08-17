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

## Download from Web

**Purpose:** download a file from a URL into a local directory. **Pins:** Exec input/output, URL and Location inputs, Result object (path, bytes, status). **Configure:** URL and destination directory. **Produces:** absolute path of the saved file, byte count, and HTTP status. **Capability:** network and file write. **Failure:** invalid URLs, HTTP errors, and write failures stop the run. **Example:** Button Trigger → Download from Web → Desktop Notification.

## Display Message

**Purpose:** show a native dialog window with an OK button and block until dismissed. **Pins:** Exec input/output, Title and Message inputs, Result object. **Configure:** title and message. **Produces:** the title/message that were shown and a Dismissed flag. **Capability:** none. **Failure:** platform dialog errors stop the run. **Example:** Button Trigger → Display Message (Title: Done, Message: Pipeline finished).

## Display Question

**Purpose:** show a native dialog with Yes/No buttons and branch on the user's choice. **Pins:** Exec input, Yes and No exec outputs, Result object. **Configure:** title and question text. **Produces:** the chosen `choice` value (`yes` or `no`). **Capability:** none. **Failure:** platform dialog errors stop the run. **Example:** Button Trigger → Display Question → Yes → HTTP Request, No → Desktop Notification.

## Display Input Dialog

**Purpose:** show a styled dialog with an input field and Continue/Cancel buttons. **Pins:** Exec input, Continue and Canceled exec outputs, Value and Result data outputs. **Configure:** title, message, field label, and input type (text or number). **Produces:** the typed Value (nil when cancelled) and a Canceled flag. **Capability:** none. **Failure:** invalid number input fails the run; cancellation routes from the Canceled pin with a nil Value. **Example:** Button Trigger → Display Input Dialog (number) → Continue → Math: Add.
