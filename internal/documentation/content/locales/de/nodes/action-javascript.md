# JavaScript

## Zweck

Führt eine kleine synchrone JavaScript-Aktion in der aktuellen Blueprint-Ausführung aus. Über **Code bearbeiten** werden typisierte Ein- und Ausgabe-Pins deklariert. Das Programm muss genau ein Objekt zurückgeben, dessen Schlüssel allen konfigurierten Ausgabe-IDs entsprechen. Werte werden nicht stillschweigend konvertiert und müssen dem deklarierten `TypeSpec` entsprechen.

## Beispiel

Deklarieren Sie die erforderliche Eingabe `name` und die Ausgabe `message` jeweils als **Text** im Typ-Picker:

```js
return { message: `Hallo, ${name}!` };
```

Die Eingabe steht auch als `inputs.name` bereit.

## Eingaben lesen

Eingabe-IDs werden zu lokalen JavaScript-Variablen: Der konfigurierte Pin
`name` ist als `name` verfügbar. Jeder Pin liegt außerdem im Objekt `inputs`,
auch für dynamischen Zugriff:

```js
const direct = name;
const property = inputs.name;
const dynamic = inputs["name"];
const label = inputs.optionalLabel ?? "Unbenannt";
return { message: `${direct}: ${label}` };
```

Pin-IDs müssen gültige JavaScript-Bezeichner wie `userName`, `count` oder
`filePath` sein. Namen wie `first-name`, `inputs` und `np` werden abgelehnt.
Eine fehlende Pflichteingabe lässt den Knoten sicher fehlschlagen; `??` ist nur
für optionale Pins gedacht.

## System-API und Sicherheit

`np` bietet nur lokale, begrenzte Helfer für Variablen, Base64, Hashes, Zusammenfassungen, Berichte, Chat und Benachrichtigungen. Datei- und Netzwerkzugriffe benötigen die im Editor aktivierte passende Berechtigung. Der Runtime werden keine Go-Objekte, Geheimnisse oder Host-Handles ausgesetzt. Die vollständige API steht im Leitfaden **JavaScript-Knoten**.
