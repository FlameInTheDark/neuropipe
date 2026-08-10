# Typzusicherung

## Zweck

Grenzt einen `any`-Wert ohne Konvertierung auf einen expliziten Blueprint-V3-Typvertrag ein. Der Knoten prüft zur Laufzeit primitive Werte, Listen, Maps und Record-Felder; bei einer Abweichung wird der Lauf sicher beendet.

## Beispiel

`Feld abrufen → Typzusicherung ({"kind":"record","fields":[{"name":"id","type":{"kind":"string"}}]}) → Objekt erstellen`.

Verwenden Sie **Umwandeln**, wenn ein primitiver Wert konvertiert werden muss. Verwenden Sie **Typzusicherung** nur, wenn der Wert den Vertrag bereits erfüllt.
