# Datum erstellen

## Zweck

Erstellt einen Datumstempel aus Jahr, Monat, Tag und Zeit. Verbundene Zahlen-Pins haben Vorrang vor den manuellen Werten im Inspektor.

## Eingaben und Ausgaben

**Jahr**, **Monat**, **Tag**, **Stunde**, **Minute**, **Sekunde** und **Millisekunde** bilden das Datum. Die Ausgaben sind **Zeitstempel (ms)** und **ISO 8601**.

## Konfiguration

**Zeitzone** legt fest, wie die Kalenderwerte interpretiert werden. Neue Knoten starten mit dem aktuellen Jahr, dem 1. Januar und 00:00 Uhr.

## Fehler und Beispiel

Der Monat muss zwischen 1 und 12 liegen. Setzen Sie Jahr `2026`, Monat `12`, Tag `31` und verbinden Sie den Zeitstempel mit `Datum formatieren`.

