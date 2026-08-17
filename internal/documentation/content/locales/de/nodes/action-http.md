# HTTP-Anfrage

## Zweck
Ruft einen HTTP-Endpunkt ab und stellt eine Text- oder JSON-Antwort dem Graphen bereit.

## Konfiguration

- **URL** und **Methode** legen Ziel und Verb der Anfrage fest.
- **Body** wird als JSON gesendet, außer ein `Content-Type`-Header ist konfiguriert.
- **Anfrage-Header** ist ein Schlüssel/Wert-Editor. Wiederholte Headernamen werden
  als getrennte Anfrage-Header gesendet.
- Aktivieren Sie **Eigenen User-Agent verwenden**, um das Feld User-Agent
  einzublenden. Es ersetzt jeden in der Header-Liste gesetzten `User-Agent`.
- **Skripte entfernen** entfernt `script`- und `noscript`-Elemente aus einem
  HTML-Antworttext; **Styles entfernen** entfernt `style`-Elemente und
  `link rel="stylesheet"`-Verweise. Beide lassen den Antworttext für
  Nicht-HTML-Antworten wie JSON unverändert.

Der Node läuft nur, wenn sein Ausführungs-Eingang gepulst wird. Header sind
Konfiguration, kein Daten-Pin, und können allein keine Anfrage auslösen.

## Ergebnis

Das Ergebnis-Objekt stellt den HTTP-Statuscode, den Antworttext, die
Antwort-Header und geparstes JSON bereit, wenn der Antworttext gültiges JSON ist.
Jede 4xx- oder 5xx-Antwort stoppt die aktuelle Ausführung und erscheint im
Ausführungsprotokoll.

## Beispiel
`Schaltflächen-Auslöser → HTTP-Anfrage → Bericht erstellen`; verbinden Sie Ergebnis mit Feld abrufen für eine JSON-Eigenschaft.
