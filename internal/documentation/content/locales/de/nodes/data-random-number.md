# Zufallszahl

## Zweck
Erzeugt bei jeder Blueprint-Ausführung eine Zufallszahl mit optionalen Bereichsgrenzen und wahlweise Float- oder Integer-Ausgabe.

## Konfiguration
- **Typ**: `float` für einen Bruchteil in `[0, 1)` (oder Ihrem Bereich), `integer` für eine ganze Zahl.
- **Bereich verwenden**: wenn aktiviert, erzeugt der Knoten Werte innerhalb von `[Von, Bis]` statt `[0, 1)`.
- **Von**: inklusive untere Grenze des Bereichs.
- **Bis**: inklusive obere Grenze des Bereichs.

## Eingänge
Die **Von**- und **Bis**-Datenpins sind optional. Wenn sie verbunden sind, haben ihre Werte Vorrang vor den Inspector-Feldern, sodass vorgelagerte Knoten den Bereich dynamisch steuern können. Wenn **Bereich verwenden** deaktiviert ist, werden die Grenzen ignoriert.

## Beispiel
`Schaltflächen-Auslöser → Zufallszahl (integer, Bereich 1–6) → Desktop-Benachrichtigung` zeigt ein Würfelergebnis. Verbinden Sie `Variable abrufen` mit **Von**, um die untere Grenze zur Laufzeit zu steuern.
