# Aus Array auswählen

## Zweck

Liest das Element an einem nullbasierten Index aus einer Liste.

## Eingaben

- **Array**: Die zu durchsuhende Liste.
- **Index**: Nullbasierter, ganzzahliger Index des Elements.

## Ausgang

- **Wert**: Das Element am angegebenen Index.

## Konfiguration und Beispiel

Verbinden Sie **Array** mit einer Liste (z. B. aus `JSON abfragen`) und **Index** mit einem Zahlen-Knoten. Ein negativer, nicht ganzzahliger oder außerhalb liegender Index schlägt mit einem Fehler fehl.

Alternativ können Sie den **Index** direkt im Inspektor des Knotens festlegen,
solange kein Index-Datenpin verbunden ist. Eine angeschlossene Verbindung
überschreibt immer den konfigurierten Wert.