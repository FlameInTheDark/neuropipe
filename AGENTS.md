# Neuropipe contribution rules

## Localization

- Every user-visible static string must be retrieved through the shared i18n
  catalog. Do not introduce hard-coded UI copy in React components, dialogs,
  empty states, placeholders, validation messages, tooltips, or accessibility
  labels.
- Add English, German, French, and Russian translations in the same change.
- Keep persisted/user-authored content untouched: pipeline names, prompts,
  reports, secrets, variables, model names, file paths, and execution data are
  data, not UI copy.
- Use locale-aware formatters for dates, numbers, durations, and plural text.
- Documentation must be real Markdown source files. Add localized Markdown
  beside its English source; use the built-in English fallback only while a
  translation is genuinely unavailable.

## UI primitives and interaction

- Never use native browser dialogs (`alert`, `confirm`, `prompt`) or native
  browser selects/tooltips for product UI. Use Neuropipe's shared styled dialog,
  `Select`, and `Tooltip` primitives.
- Use Lucide icons through the existing icon utilities. Do not introduce a
  competing icon set or emoji as interface icons.
- New controls must be keyboard-accessible, correctly labelled, viewport-safe,
  and preserve focus appropriately after dialogs and menus close.
- Reuse shared UI components instead of recreating menus, dropdowns, tooltips,
  confirmation flows, Markdown renderers, or date controls in a view.
- Use `frontend/src/components/ContextMenu.tsx` for every product context menu.
  Do not reimplement cursor placement, viewport clamping, outside-click/Escape
  dismissal, or focus restoration in an individual view; supply only that
  view's menu actions through the shared component.

## Architecture and quality

- Follow SOLID, DRY, and KIS. Keep Wails as the sole boundary between React and
  desktop capabilities; React must never directly access files, processes,
  secrets, or provider endpoints.
- Prefer small domain interfaces, explicit errors, cancellable work, and owned
  lifecycle management. Do not leave unmanaged goroutines, timers, listeners,
  or child processes.
- Preserve local-first privacy: never log prompts, responses, payloads,
  secrets, credentials, URLs, or IP addresses unless the feature explicitly
  stores approved, redacted local data.
- Treat existing workspace changes as user-owned. Avoid destructive commands
  and preserve unrelated edits.
- Verify changes proportionately: Go formatting/tests/vet, TypeScript checks,
  and a production frontend build for UI work. Regenerate Wails bindings when
  bound Go contracts change.
