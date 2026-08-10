# LLM-Werkzeugfunktionen

Eine LLM-Werkzeugfunktion ist eine veröffentlichte, wiederverwendbare Blueprint-Funktion. Ein **Agent** oder **Coding-Agent** kann sie über seinen Pin **Tools** aufrufen. Sie ist kein Ausführungsknoten: Ihr einzelner Tool-Ausgang beschreibt nur die Verfügbarkeit für das Modell. Der Host prüft den Aufruf, führt die Funktion aus und gibt das typisierte Ergebnis an das Modell zurück.

## Zuerst den Vertrag erstellen

1. Wählen Sie unter **Funktionen**: **Neue Funktion → LLM-Werkzeug**.
2. Beschreiben Sie, *wann* das Modell das Werkzeug verwenden soll. Beispiel: „Aktuelle Vorhersage für eine Stadt abrufen. Nur bei Wetterfragen verwenden.“
3. Fügen Sie öffentliche Ein- und Ausgaben hinzu. Jeder Pin braucht einen **Modellhinweis** mit Bedeutung, Einschränkungen und Beispiel.
4. Wählen Sie konkrete Typen, markieren Sie nur tatsächlich nötige Eingaben als Pflicht und veröffentlichen Sie erst mit einem erreichbaren Pfad **Function Entry → Function Return**.

Die Veröffentlichung verlangt eine Funktionsbeschreibung, mindestens eine beschriebene Ausgabe, Hinweise für alle Pins, konkrete Typen und eindeutige Argumentnamen. Ein Werkzeug darf keine Eingaben haben, muss aber ein beschriebenes Ergebnis liefern.

## Beispiel: Wetter abfragen

Erstellen Sie **Stadtvorhersage abrufen**:

| Teil | Vertrag |
| --- | --- |
| Beschreibung | „Aktuelle Vorhersage für eine Stadt abrufen. Bei Wetterfragen verwenden.“ |
| Eingabe `city` | Text, Pflicht. Hinweis: „Stadt und Land, zum Beispiel `Yekaterinburg, RU`.“ |
| Ausgabe `forecast` | Text. Hinweis: „Kurze aktuelle Vorhersage mit Zustand und Temperatur.“ |

Im Funktionsgraphen:

```text
Function Entry ──exec──> HTTP Request ──exec──> Function Return
      city ──data──> HTTP-Zuordnung ──data──> forecast
```

Veröffentlichen Sie die Funktion und verbinden Sie ihren **Tool**-Ausgang im Pipeline-Editor mit **Tools** eines Agenten. Mehrere unabhängige Werkzeuge können am selben Tools-Pin hängen.

Beispiel für Agent-Anweisungen:

> Beantworte die Wetterfrage. Rufe bei einer aktuellen Vorhersage das verbundene Werkzeug Stadtvorhersage abrufen mit Stadt und Land auf. Erfinde keine Vorhersage.

## Typen an der Modellgrenze

Werkzeugargumente sind JSON. Neuropipe dekodiert sie vor der Ausführung in den exakten Go-inspirierten `TypeSpec`: Text und Boolean bleiben JSON-Werte, Ganzzahlen müssen ohne Nachkommastellen sein, Bytes werden als Base64-Zeichenfolge zu `[]byte`, Listen prüfen jedes Element, Maps brauchen Textschlüssel und anonyme Records prüfen ihre deklarierten Felder.

`any`, benannte Go-Records und Maps mit Nicht-Text-Schlüsseln sind keine gültigen öffentlichen Werkzeugverträge. Es gibt keine stillen Text-zu-Zahl-, Ganzzahl-zu-Float- oder Bytes-zu-Text-Konvertierungen. Verwenden Sie dafür explizite Umwandlungsknoten im Graphen.

## Aufrufe und Sicherheit

Der Agent ist durch seine konfigurierte Rundenzahl begrenzt. Unbekannte Argumente, fehlende Pflichtwerte, falsche Typen, ungültiges Base64 oder unbekannte Record-Felder werden als sicherer Vertragsfehler an das Modell zurückgegeben, damit es den Aufruf korrigieren kann. Interne Fehler mit lokalen Pfaden, Geheimnissen oder Nutzdaten werden nicht weitergegeben.

Die Funktion benötigt weiterhin nur die Fähigkeiten ihrer inneren Knoten. Die normale lokale Vertrauens- und Freigabeprüfung bleibt aktiv. Speichern Sie keine Zugangsdaten in Beschreibung, Modellhinweisen, Agent-Anweisungen oder Werkzeugergebnissen.
