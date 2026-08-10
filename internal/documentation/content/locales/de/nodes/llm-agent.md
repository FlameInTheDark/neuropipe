# Agent

## Zweck

Erledigt eine begrenzte Aufgabe nur mit ausdrücklich verbundenen, veröffentlichten LLM-Werkzeugfunktionen.

## Tools-Pin

Der Eingang **Tools** ist ein deklarativer, unbegrenzter Werkzeug-Pin und kein Exec-Pin. Verbinden Sie den einzelnen Ausgang **Tool** jeder veröffentlichten LLM-Werkzeugfunktion damit. Der Agent stellt dem Anbieter nur diese Funktionen bereit; eine nicht verbundene Funktion kann nicht aufgerufen werden.

Jeder Aufruf wird gegen Namen, konkrete Eingabetypen, Pflichteingaben und Modellhinweise der veröffentlichten Funktion geprüft. Das Funktionsergebnis geht als JSON in die nächste Modellrunde. Fehlerhafte Argumente erhalten sichere Vertragsrückmeldung, damit das Modell innerhalb seines Rundenlimits erneut versuchen kann.

## Beispiel

`Stadtvorhersage abrufen.Tool → Agent.Tools`, dann `Schaltflächen-Auslöser → Agent → Bericht erstellen`.
