# Nachricht anzeigen

## Zweck
Zeigt ein natives Dialogfenster mit Titel, Nachricht und OK-Schaltfläche. Die Ausführung wird blockiert, bis der Benutzer den Dialog schließt, und wird dann vom Dann-Pin fortgesetzt.

## Konfiguration
- **Titel**: Text in der Titelleiste des Dialogs.
- **Nachricht**: Text im Dialogkörper. Mehrzeiliger Text wird unterstützt.

## Beispiel
`Schaltflächen-Auslöser → Nachricht anzeigen (Titel: Fertig, Nachricht: Pipeline abgeschlossen)`. Verbinden Sie `Text formatieren` mit dem **Nachricht**-Pin, um dynamische Werte anzuzeigen.
