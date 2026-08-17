# Eingabedialog anzeigen

## Zweck
Zeigt ein formatiertes Dialogfenster mit Titel, Nachricht, beschriftetem Eingabefeld sowie Weiter-/Abbrechen-Schaltflächen. Die Ausführung wird blockiert, bis der Benutzer antwortet. Weiter gibt den typisierten Wert auf dem Wert-Pin aus und setzt vom Weiter-Pin fort; Abbrechen setzt vom Abgebrochen-Pin fort und gibt nil auf dem Wert-Pin aus.

## Konfiguration
- **Titel**: Text in der Titelleiste des Dialogs.
- **Nachricht**: Text im Dialogkörper, typischerweise eine Aufforderung zur erwarteten Eingabe.
- **Feldbezeichnung**: Beschriftung neben dem Eingabefeld.
- **Eingabetyp**: `text` akzeptiert jede Zeichenkette, `number` parst die Eingabe als Float und schlägt bei ungültiger Eingabe fehl. Der Wert-Ausgangspin folgt diesem Typ.

## Beispiel
`Schaltflächen-Auslöser → Eingabedialog anzeigen (number) → Weiter → Mathematik: Addieren (Wert-Pin verwenden), Abgebrochen → Desktop-Benachrichtigung`. Der Wert-Pin ist `nil`, wenn der Benutzer abbricht.
