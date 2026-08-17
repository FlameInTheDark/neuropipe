# Frage anzeigen

## Zweck
Zeigt ein natives Dialogfenster mit Ja- und Nein-Schaltflächen. Die Ausführung wird blockiert, bis der Benutzer eine Wahl trifft, und wird dann vom passenden Exec-Pin fortgesetzt, sodass der Graph nach der Entscheidung verzweigen kann.

## Konfiguration
- **Titel**: Text in der Titelleiste des Dialogs.
- **Nachricht**: Text im Dialogkörper. Formulieren Sie ihn als Ja-/Nein-Frage.

## Beispiel
`Schaltflächen-Auslöser → Frage anzeigen (Nachricht: Bericht jetzt senden?) → Ja → HTTP-Anfrage, Nein → Desktop-Benachrichtigung (übersprungen)`. Die Ergebnis-Ausgabe meldet, welche Schaltfläche der Benutzer gedrückt hat (`yes` oder `no`).
