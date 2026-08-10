# Base64 kodieren

## Zweck

Kodiert eine ausgewählte Eingabe **Bytes** oder **Text** explizit als Base64.
Wählen Sie Ein- und Ausgabe-Repräsentation unabhängig. Ein Bytes-Ausgang ist
die Base64-Bytefolge, Text ist ihre UTF-8-Stringform.

## Beispiel

`Datei lesen (Bytes) → Base64 kodieren → HTTP-Anfrage`.

Jede Auswahl ändert den exakten Wire-Typ. Verbundene Werte werden weder
still interpretiert noch geparst oder konvertiert.
