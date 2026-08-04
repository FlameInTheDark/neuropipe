# Objekt aufteilen

Teilt ein Objekt über konfigurierbare, typisierte Ausgabepins auf.

## Konfiguration

Verbinden Sie ein Objekt mit **Quelle**. Jede Ausgabe hat eine stabile ID,
einen sichtbaren Namen, einen Punktpfad und einen Datentyp. `kunde.name` liest
einen verschachtelten Wert; `items.0.name` kann ein Listenelement lesen.

Bei bekannten Ergebnissen von Erstpartei-Knoten erstellt **Automatisch
einrichten** alle dokumentierten Felder. Die Zuordnungen bleiben danach frei
bearbeitbar.

## Beispiel

Terminalergebnis → Objekt aufteilen (`terminal.output`) → LLM-Prompt.
