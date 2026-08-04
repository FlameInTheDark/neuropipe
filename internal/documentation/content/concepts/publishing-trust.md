# Publishing and trust

A draft is editable and safe to test manually. Publishing validates pin compatibility, reachability, trigger configuration, function dependencies, and capabilities, then creates an immutable revision.

## Trust

Sensitive nodes declare capabilities such as network access, file roots, terminal, Git, or plugin execution. Manual runs show an approval preview when needed. Schedules and webhooks run unattended only when their published revision has an explicit matching trust grant.

Editing and publishing changes the revision. Trust does not silently move to that new revision. Regrant it after reviewing what changed.

## Migration

Blueprint-v2 catalog upgrades preserve historical revisions and execution history. Directly convertible drafts are copied to a new reviewable draft; ambiguous graphs are paused and show a migration issue. Review, repair if necessary, and publish before using their triggers again.
