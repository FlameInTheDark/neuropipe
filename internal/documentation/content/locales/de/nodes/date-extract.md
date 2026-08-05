# Komponenten extrahieren

## Zweck

Liest Kalender-, Uhrzeit-, ISO- und Unix-Werte aus einem Datumstempel. Der reine Knoten führt keine Aktion aus.

## Eingabe und Ausgaben

Verbinden Sie **Zeitstempel (ms)** von `Jetzt`, `Datum erstellen`, `Datum parsen` oder einer Dauerrechnung. Ausgegeben werden Jahr, Monat, Tag, Uhrzeit, Wochentag (`0` ist Sonntag), Tag und ISO-Woche des Jahres, ISO-Text sowie Unix-Sekunden und -Millisekunden.

## Konfiguration und Beispiel

**Zeitzone** bestimmt die dargestellten Komponenten. Verbinden Sie **Stunde** mit einem Zahlenvergleich und dessen Boolean-Ausgabe mit `Branch.Bedingung`.

