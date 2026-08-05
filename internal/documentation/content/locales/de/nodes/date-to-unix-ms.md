# In Unix-Millisekunden umwandeln

## Zweck

Kennzeichnet einen Datumstempel ausdrücklich als Unix-Millisekunden. Neuropipe verwendet dieses Format bereits intern, daher ist der reine Knoten ein lesbarer Durchgang.

## Eingabe und Ausgabe

**Zeitstempel (ms)** muss eine endliche Zahl sein. **Unix-Millisekunden** gibt dieselbe Zahl zurück.

## Beispiel

Verwenden Sie den Knoten vor einem HTTP-Aufruf, dessen API-Feld `unix_ms` heißt.

