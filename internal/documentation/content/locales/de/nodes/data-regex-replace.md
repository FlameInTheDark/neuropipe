# Regex-Ersetzen

## Zweck

Ersetzt jeden Go-RE2-Treffer in **Text**. **Muster** und **Ersetzung** können
im Inspektor konfiguriert oder über Text-Kabel geliefert werden; ein verbundenes
Kabel hat Vorrang.

## Ausgaben

- **Text** ist der ersetzte Text.
- **Ersetzungen** ist die genaue ganzzahlige Anzahl der Treffer.
- **Geändert** zeigt, ob sich der Ausgabetext vom Eingabetext unterscheidet.

Ein fehlender Treffer ist kein Fehler. Der ursprüngliche Text wird mit null
Ersetzungen und Geändert=false zurückgegeben.

## Beispiel

Mit Text `Ada Lovelace`, Muster `(?P<first>\w+) (?P<last>\w+)` und Ersetzung
`${last}, $1` lautet der Ausgabetext `Lovelace, Ada`.

## RE2- und Ersetzungssyntax

Muster verwenden die sichere RE2-Syntax von Go. Die Ersetzung folgt
`regexp.ReplaceAllString`: `$1` ist die erste Erfassungsgruppe und `${name}`
eine benannte Gruppe. Lookaround und Rückreferenzen im Muster werden nicht
unterstützt. Ungültige Muster lassen den Knoten fehlschlagen; Werte werden nie
implizit umgewandelt.
