# Jetzt

## Zweck

Erzeugt den aktuellen Zeitpunkt als Datumstempel. Der Knoten ist rein, hat keine Exec-Pins und wird nur innerhalb des aktiven Laufs ausgewertet.

## Ausgaben

- **Zeitstempel (ms)**: Millisekunden seit der Unix-Epoche.
- **ISO 8601**: RFC3339-Text des Zeitpunkts.
- **Lokaler Text**: Lesbare Darstellung im gewählten Zeitzonenmodus.

## Konfiguration und Beispiel

Wählen Sie bei **Zeitzone** `local` oder `utc`. Verbinden Sie **Zeitstempel (ms)** mit `Daten vergleichen.Links (ms)` und den Ausgang **Nach** mit einer Branch-Bedingung.

