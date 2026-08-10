# Regex-Aufteilen

## Zweck

Teilt **Text** an jedem Go-RE2-Treffer. Konfigurieren Sie **Muster** im
Inspektor oder verbinden Sie ein Text-Kabel, um den gespeicherten Wert für den
aktuellen Lauf zu überschreiben.

## Ausgaben

- **Teile** ist ein genaues `list[string]`.
- **Trennungen** ist die genaue ganzzahlige Anzahl der Trennzeichen-Treffer.
- **Treffer vorhanden** zeigt, ob das Muster im Eingabetext vorkommt.

Führende und abschließende leere Teile bleiben erhalten. Ohne Treffer enthält
Teile genau den ursprünglichen Text, Trennungen ist null und Treffer vorhanden
ist false.

## Beispiel

Verwenden Sie Text `one, two; three` mit Muster `[,;]\s*`. Teile ist
`["one", "two", "three"]` und kann direkt an **For Each** gehen.

## RE2-Syntax

Muster verwenden die sichere RE2-Syntax von Go, nicht PCRE. Lookahead,
Lookbehind und Rückreferenzen im Muster werden nicht unterstützt. Ungültige
Ausdrücke schlagen explizit fehl; Nicht-Text wird nie in Text umgewandelt.
