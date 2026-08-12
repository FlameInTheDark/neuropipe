# Globale Variable abrufen

## Zweck
Liest eine werkstattweite Variable, die über alle Pipelines und Läufe hinweg
geteilt wird. Ein Lesevorgang vor dem ersten Schreiben liefert den
deklarierten Standardwert der Variablen.

Die Variable wird aus einer Auswahlliste der auf dem Bildschirm **Variablen**
verwalteten Namen gewählt. Lesevorgänge sind über gleichzeitig laufende
Pipelines hinweg sicher.

## Beispiel
`Globale Variable abrufen (visits) → Addition → Globale Variable setzen`, um
einen Zähler über Läufe hinweg zu aggregieren.
