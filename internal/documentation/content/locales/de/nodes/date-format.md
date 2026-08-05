# Datum formatieren

## Zweck

Wandelt einen Datumstempel für Berichte, Chat-Antworten, Benachrichtigungen oder HTTP-Werte in Text um.

## Eingaben

**Zeitstempel (ms)** ist erforderlich. Der Text-Pin **Format** kann das Inspektorformat überschreiben.

## Konfiguration

Das Format verwendet Go-Referenzzeitlayouts, keine `YYYY`-Token: `2006-01-02` erzeugt ein Datum und `15:04` eine Uhrzeit. **Zeitzone** wählt `local` oder `utc`.

## Fehler und Beispiel

Der Zeitstempel muss eine endliche Zahl sein. `Jetzt.Zeitstempel (ms)` → **Zeitstempel (ms)** und Format `2006-01-02` erzeugen einen Datumstext.

