# An Array anhängen

## Zweck

Hängt an eine Liste an und liefert die neue Liste. Der Knoten ist rein und verändert das verbundene Array nicht; die Eingabe-Liste bleibt anderweitig wiederverwendbar.

Der Modus **Anhängen** legt fest, was der Wert-Eingang bedeutet: **Einzelnes Element** hängt den Wert als ein Element an — eine hier angeschlossene Liste wird als verschachteltes Einzelelement eingefügt. **Array-Elemente** verkettet: die Elemente der angeschlossenen Liste werden einzeln angehängt, sodass `[3, 4]` an `[1, 2]` das Ergebnis `[1, 2, 3, 4]` liefert.

## Eingänge

- **Array**: List, die erweitert wird.
- **Wert**: Element oder Liste, die angehängt wird.

## Ausgang

- **Array**: Neue Liste mit dem Angehängten.

## Konfiguration und Beispiel

Verbinden Sie **Array** mit einem `Daten: JSON parsen`-Ausgang und **Wert** mit einem Konstanten-Knoten. Für das Zusammenführen zweier Listen wählen Sie **Array-Elemente** und verbinden **Wert** mit der zweiten Liste. Mehrere „An Array anhängen“-Knoten nacheinander bauen eine Liste Element für Element auf. Ist die Eingabe **Array** keine Liste, schlägt der anfordernde Ausführungspfad fehl; im Modus **Array-Elemente** muss auch **Wert** eine Liste sein.
