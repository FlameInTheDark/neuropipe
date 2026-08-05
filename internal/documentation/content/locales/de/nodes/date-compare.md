# Daten vergleichen

## Zweck

Vergleicht zwei Datumstempel und erzeugt Boolean-Werte für Blueprint-Steuerfluss.

## Eingaben und Ausgaben

**Links (ms)** und **Rechts (ms)** müssen endliche Zahlen sein. **Vor**, **Nach** und **Gleich** sind gegenseitig ausschließende Boolean-Ausgaben. **Differenz** ist `Links − Rechts` in Millisekunden, Sekunden, Minuten, Stunden und Tagen.

## Beispiel

Verbinden Sie `Jetzt` links und `Datum erstellen` rechts. Der Ausgang **Nach** kann `Branch.Bedingung` steuern.

