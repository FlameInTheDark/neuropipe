# Base64 dekodieren

## Zweck

Dekodiert eine ausgewählte Base64-Repräsentation **Text** oder **Bytes**. Die
Auswahl der Ausgabe legt ursprüngliche Bytes oder UTF-8-Text fest.

## Beispiel

`Feld abrufen → Base64 dekodieren → Datei schreiben`.

Fehlerhaftes Base64 beendet den aktiven Pfad sicher. Text für Binärdaten
schlägt ebenfalls sicher fehl; verwenden Sie dafür Bytes.
