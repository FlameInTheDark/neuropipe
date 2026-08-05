# Datum parsen

## Zweck

Wandelt Datumstext in einen Neuropipe-Zeitstempel in Millisekunden um.

## Eingaben und Konfiguration

**Text** ist erforderlich; **Format** kann per Pin oder im Inspektor ein Go-Referenzzeitlayout angeben. Ohne Format probiert Neuropipe gängige RFC3339-, ISO- und Datumsformate. **Zeitzone** gilt für Text ohne eigene Zeitzone.

## Fehler und Beispiel

Für mehrdeutige Werte wie `06/07/2026` setzen Sie ein explizites Format. Verbinden Sie den Ausgang **Zeitstempel (ms)** mit `Daten vergleichen.Links (ms)`.

