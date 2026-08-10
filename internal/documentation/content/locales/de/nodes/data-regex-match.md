# Regex-Abgleich

## Zweck

Prüft **Text** mit einem regulären Go-RE2-Ausdruck und gibt jeden Treffer als
typisierte Struktur zurück. Verbinden Sie Text und konfigurieren Sie **Muster**
im Inspektor, oder verbinden Sie ein Text-Kabel mit Muster, um den gespeicherten
Wert für diesen Lauf zu überschreiben.

## Ausgaben

- **Treffer vorhanden** ist wahr, wenn mindestens ein Treffer existiert.
- **Anzahl** ist die genaue ganzzahlige Anzahl der Treffer.
- **Treffer** ist `list[RegexMatch]` mit `text`, `startByte`, `endByte` und
  `captures`.

Jede Erfassungsgruppe ist ein `RegexCapture` mit einem einsbasierten `index`,
ihrem `name` (bei unbenannten Gruppen leer), `matched`, `text`, `startByte` und
`endByte`. Nicht beteiligte optionale Gruppen haben sichere Werte: `matched`
ist false, Text ist leer und beide Offsets sind `-1`. Offsets sind UTF-8-
Bytepositionen wie im Go-Paket `regexp`.

Ein fehlender Treffer ist kein Fehler: Treffer vorhanden ist false, Anzahl ist
null und Treffer ist eine leere Liste.

## Beispiel

Verwenden Sie das Muster `(?P<name>\w+)=(?P<value>\d+)` mit dem Text
`limit=25 retries=3`. Treffer enthält zwei Datensätze. Der erste enthält
`limit=25` sowie die benannten Gruppen `name` (`limit`) und `value` (`25`).

## RE2-Syntax

Muster verwenden die sichere RE2-Engine von Go. Benannte Gruppen verwenden
`(?P<name>...)`. Lookahead, Lookbehind und Rückreferenzen im Muster werden
nicht unterstützt. Ungültige Muster lassen den Knoten fehlschlagen, ohne Werte
umzuwandeln oder zu erraten.
