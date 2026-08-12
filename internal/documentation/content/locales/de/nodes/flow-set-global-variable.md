# Globale Variable setzen

## Zweck
Schreibt eine werkstattweite Variable, die über alle Pipelines und Läufe
hinweg geteilt wird. Der Wert ist sofort im Speicher verfügbar und wird
höchstens einmal pro Sekunde in die lokale Datenbank geschrieben, sodass er
einen Neustart der Anwendung überdauert.

Der Knoten führt eine von drei Operationen aus, ausgewählt im Inspektor:

- **Setzen** überschreibt den Wert und validiert ihn gegen den deklarierten
  Datentyp.
- **Erhöhen** addiert atomar eine Zahl und verhindert verlorene Updates, wenn
  zwei Pipelines gleichzeitig laufen.
- **Anhängen** fügt atomar ein Element an eine Liste an.

## Beispiel
`Zeitplan-Auslöser → Globale Variable setzen (lastRun, Operation: Setzen)`, um
den letzten Lauf einer Wartungspipeline zu erfassen.
