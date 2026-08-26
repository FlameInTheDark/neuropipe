# Pipelines auflisten

## Zweck
Gibt alle veröffentlichten Pipelines des Workspaces als strukturierte Daten
aus, damit ein Ablauf den Katalog selbst auswerten kann – etwa um ein Ziel
für den Run-Pipeline-Knoten zu wählen oder Namen in Berichte zu speisen.

## Ausgaben
- **Then**: Exec-Fortsetzung, nachdem die Liste gesammelt wurde.
- **Pipelines**: Liste von Objekten mit `id`, `name`, `description`,
  `status` und `publishedRevision`. Nur Entwürfe werden nicht eingeschlossen.
- **Count**: Anzahl der Einträge.

## Konfiguration
Dieser Knoten hat keine Einstellungen; der Katalog wird bei jeder Ausführung
live gelesen.

## Beispiel
`Manueller Trigger → Pipelines auflisten → JavaScript (nach Name filtern) → Run Pipeline`