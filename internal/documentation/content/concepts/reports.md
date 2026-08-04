# Reports

**Create Report** stores Markdown in local SQLite. The Reports view supports feed and side-by-side layouts, saved view preference, tags, text search, date-and-time range filters, and a context-menu delete action.

Each report records its title, tags, source pipeline, execution ID, creation time, and the time the originating run started. The report renderer is the same safe Markdown renderer used by chat, model cards, and documentation.

Create reports for human-facing summaries rather than large binary artifacts. Keep secrets out of report text; report content is intentionally readable in the local workspace.
