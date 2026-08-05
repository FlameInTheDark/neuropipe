# In Unix-Sekunden umwandeln

## Zweck

Wandelt den Millisekunden-Zeitstempel von Neuropipe in Unix-Sekunden für APIs um.

## Eingabe und Ausgabe

**Zeitstempel (ms)** muss eine endliche Zahl sein. **Unix-Sekunden** ist der Wert geteilt durch `1000`; vorhandene Millisekunden bleiben als Bruchteil erhalten.

## Beispiel

Verbinden Sie `Jetzt.Zeitstempel (ms)` und übergeben Sie den Ausgang an einen HTTP-Parameter, der einen Unix-Epochenwert erwartet.

