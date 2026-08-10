# An Array anhängen

## Zweck

Hängt einen Wert an eine Liste an und liefert die neue Liste. Der Knoten ist rein und verändert das verbundene Array nicht; die Eingabe-Liste bleibt anderweitig wiederverwendbar.

## Eingänge

- **Array**: List, die erweitert wird.
- **Wert**: Element, das angehängt werden soll.

## Ausgang

- **Array**: Neue Liste mit dem angehängten Element.

## Konfiguration und Beispiel

Verbinden Sie **Array** mit einem `Daten: JSON parsen`-Ausgang und **Wert** mit einem Konstanten-Knoten. Mehrere „An Array anhängen“-Knoten nacheinander bauen eine Liste Element für Element auf. Ist die Eingabe **Array** keine Liste, schlägt der anfordernde Ausführungspfad fehl.