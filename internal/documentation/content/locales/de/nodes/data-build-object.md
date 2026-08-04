# Objekt erstellen

Erstellt aus beliebig vielen konfigurierten Datenpins ein typisiertes Objekt.
Jede Zeile in **Felder** besitzt eine stabile Pin-ID; das Umbenennen des Pins
oder seines Objektschlüssels trennt vorhandene Leitungen nicht.

## Konfiguration

Legen Sie für jeden Wert einen Pin-Namen, Datentyp und Objektschlüssel fest.
Punktpfade wie `kunde.name` erzeugen verschachtelte Objekte. Schlüssel müssen
nicht leer, eindeutig und ohne Überschneidung sein: `kunde` und `kunde.name`
sind zusammen ungültig.

## Beispiel

**Name** → `kunde.name` und **E-Mail** → `kunde.email` erzeugen:

~~~json
{"kunde":{"name":"Ada Lovelace","email":"ada@example.com"}}
~~~
