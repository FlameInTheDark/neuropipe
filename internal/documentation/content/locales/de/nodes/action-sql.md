# SQL

## Zweck

Führt eine SQL-Anweisung gegen eine registrierte lokale SQLite-Datenbank aus.
Wählen Sie eine Datenbank, geben Sie SQL ein und definieren Sie benannte
Parameter. Parameterwerte werden über typisierte Eingangspins gebunden.

## Parameter und Ergebnisse

Verwenden Sie Namen wie `:userId` und konfigurieren Sie den Parameter `userId`.
Die Ausgänge sind `Columns`, `Rows`, `Rows affected`, `Last insert ID` und
`Truncated`, gefolgt vom Ausführungs-Ausgang `Then`.

Abfragen liefern höchstens 500 Zeilen. Positions-Platzhalter und mehrere
Anweisungen werden abgelehnt.
