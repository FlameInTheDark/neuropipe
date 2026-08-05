# Dauer hinzufügen

## Zweck

Addiert Kalender- und Zeiteinheiten zu einem Datumstempel. Jahre, Monate und Tage sind Kalendereinheiten; Stunden bis Millisekunden sind verstrichene Zeit.

## Eingaben und Ausgaben

Verbinden Sie **Zeitstempel (ms)** und geben Sie bei Bedarf Jahre bis Millisekunden per Pin oder Inspektor an. Ausgaben sind der neue **Zeitstempel (ms)** und **ISO 8601**.

## Konfiguration und Beispiel

**Zeitzone** wird bei Kalenderarithmetik verwendet. `Jetzt` plus 7 Tage lässt sich anschließend mit `Datum formatieren` als Erinnerungstermin anzeigen.

