# JavaScript-Knoten

Der **JavaScript**-Knoten eignet sich für kleine, deterministische Transformationen oder lokale Orchestrierung. Er verwendet den reinen Go-Interpreter Goja, läuft synchron und gibt keine Go-Objekte direkt frei.

## Typisierten Schritt erstellen

1. Fügen Sie **Code → JavaScript** hinzu und wählen Sie **Code bearbeiten**.
2. Fügen Sie Ein- und Ausgaben hinzu. Ihre IDs werden zu JavaScript-Variablen; alle Werte sind zusätzlich über `inputs` verfügbar.
3. Wählen Sie den Typ jedes Pins im visuellen Picker. Er unterstützt primitive Typen, Listen mit typisierten Elementen, Maps mit Textschlüsseln und typisierten Werten sowie strukturelle Objekte mit benannten Pflicht- oder optionalen Feldern. Neuropipe speichert den daraus entstehenden `TypeSpec`; Sie bearbeiten kein JSON.
4. Geben Sie ein Objekt mit allen konfigurierten Ausgaben zurück. Jeder verschachtelte Wert wird geprüft; es gibt keine stillen Konvertierungen.

```js
const titles = tasks.filter((task) => !task.done).map((task) => task.title);
return { titles, count: titles.length };
```

## Eingabe-Pins im Code lesen

Jede konfigurierte Eingabe-ID wird zu einer lokalen JavaScript-Variablen. Eine Eingabe namens `name` lesen Sie also mit `name`; außerdem stehen alle Eingaben im Objekt `inputs` bereit. Das ist für dynamische Schlüssel nützlich:

```js
const greeting = `Hallo, ${name}!`;
const sameName = inputs.name;
const selected = inputs["name"];
return { greeting };
```

Verwenden Sie für jede Pin-ID einen gültigen JavaScript-Bezeichner wie `userName`, `count` oder `filePath`. IDs wie `first-name`, `inputs` und `np` werden abgelehnt. Fehlt eine erforderliche Eingabe, wird der Knoten sicher gestoppt. Für optionale Eingaben nutzen Sie bewusst einen Fallback, etwa `const label = inputs.label ?? "Unbenannt"`.

## `np`-API

`np` bietet `context`, `uuid`, `assert`, `fail`, Laufvariablen, explizite Base64- und SHA-256-Helfer, Zusammenfassungen von Pipelines/Funktionen/Ausführungen, Berichte, Chat-Helfer und Desktop-Benachrichtigungen. `np.files.list/readBytes/readText/writeBytes/writeText` benötigt Datei-Lese- oder Schreibrechte. `np.http.request({ url, method?, headers?, body? })` benötigt Netzwerkanfragen und liefert `status`, `headers` und Text-`body`.

## Berechtigungen und Grenzen

Jede aktivierte Berechtigung erscheint beim Veröffentlichen im Vertrauensdialog und wird zur Laufzeit erneut geprüft. Kein Script kann durch Importe, Pfade oder versteckte Host-APIs Rechte erhalten. Code wird vor dem Speichern geprüft; asynchrones Warten, Paketimporte, Prozesse, Geheimnisse und Go-Handles stehen nicht zur Verfügung.
