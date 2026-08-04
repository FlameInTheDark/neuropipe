# Multiplizieren

## Zweck

Multipliziert zwei Zahlen. Der reine Knoten wird nur bei Anforderung seines Result-Pins berechnet und hat keine Ausführungspins.

## Pins

**A** und **B** sind Zahleneingaben; **Result** ist die Zahlenausgabe.

Sie können **A** und **B** direkt im Inspektor setzen, wenn die Pins nicht verbunden sind. Eine verbundene Zahlenleitung hat Vorrang; beide manuellen Werte beginnen bei `0`.

## Fehler und Beispiel

Beide Eingaben müssen endliche Zahlen sein; ein Überlauf wird abgewiesen. `Preis` × `Konstante 1,2` ergibt den angepassten Preis.
