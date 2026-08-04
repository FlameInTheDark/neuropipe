# Ihr erstes Plugin erstellen

Diese Anleitung erstellt ein minimales Bundle, das Neuropipe erkennt und dessen
Markdown-Seite in der Dokumentation erscheint. Es demonstriert die derzeit
unterstützte v1-Oberfläche: Erkennung, Diagnosen und Dokumentation. Es erstellt
noch keinen ausführbaren Canvas-Knoten.

## Voraussetzungen

- Neuropipe mit **Einstellungen → Erweiterungen**.
- Go 1.26 oder eine kompatible Go-Toolchain.
- Ein beschreibbarer Plugin-Ordner aus den Einstellungen.

## 1. Ordner anlegen

~~~text
acme-status/
  plugin.json
  sidecar/
    main.go
  docs/
    status-check.md
~~~

Alle relativen Pfade werden vom Ordner mit plugin.json aus aufgelöst.

## 2. Harmlosen Sidecar bauen

Erstellen Sie sidecar/main.go:

~~~go
package main

func main() {
    select {}
}
~~~

Im Bundle-Ordner bauen:

~~~powershell
go build -o sidecar.exe ./sidecar
~~~

Der Sidecar tut absichtlich nichts. Neuropipe prüft heute nur seine Existenz
und startet ihn nicht. Ein echter künftiger Sidecar verwendet den verwalteten,
abbrechbaren gRPC-Lebenszyklus aus dem [Plugin-System](docs:extensions/plugin-system).

## 3. plugin.json erstellen

~~~json
{
  "id": "acme-status",
  "name": "Acme Status",
  "version": "0.1.0",
  "description": "Beispiel-Status-Plugin.",
  "apiVersion": "v1",
  "executable": "sidecar.exe",
  "nodes": [
    {
      "id": "status-check",
      "kind": "action",
      "label": "Status Check",
      "description": "Beispieldeklaration, derzeit nicht ausführbar.",
      "icon": "heart-pulse",
      "color": "#60a5fa",
      "capabilities": ["network"],
      "outputs": [{"id": "result", "label": "Result", "kind": "data"}],
      "fields": [{"name": "url", "label": "URL", "kind": "string", "required": true, "secret": false}]
    }
  ],
  "documentation": [
    {
      "id": "status-check",
      "title": "Status Check",
      "categoryPath": ["Extensions", "Acme Status"],
      "path": "docs/status-check.md"
    }
  ]
}
~~~

Erforderlich sind id, name, apiVersion v1 und executable. Version und
Beschreibung verbessern spätere Diagnosen. Fields, Outputs und Capabilities
zeigen die spätere Form, erzeugen aber heute keinen Bibliotheksknoten.

## 4. Dokumentation schreiben

Erstellen Sie docs/status-check.md:

~~~markdown
# Status Check

Dieses Beispiel fügt eine sichere lokale Markdown-Seite hinzu.

## Beispiel

Status Check → Create Report
~~~

Der Pfad muss im Bundle bleiben und die Datei darf höchstens 1 MiB groß sein.

## 5. Neu erkennen

1. Öffnen Sie **Einstellungen → Erweiterungen**.
2. Prüfen Sie den Plugin-Ordner.
3. Klicken Sie **Plugins neu erkennen**.
4. Prüfen Sie Acme Status, Version 0.1.0, einen deklarierten Knoten und
   **Healthy**.
5. Öffnen Sie **Dokumentation → Erweiterungen → Acme Status → Status Check**.

Nach einer Manifest- oder Dokumentationsänderung erneut neu erkennen.

## Fehlerbehebung

| Symptom | Lösung |
| --- | --- |
| Kein Eintrag | Plugin-Ordner korrigieren und neu erkennen. |
| Invalid manifest | JSON, id/name oder v1 korrigieren. |
| Sidecar unavailable | sidecar.exe bauen oder Pfad korrigieren. |
| Healthy, keine Dokumentation | Relativen .md-Pfad, categoryPath, id und Größenlimit prüfen. |
| Kein Bibliotheksknoten | Erwartetes aktuelles Verhalten; die Runtime registriert/führt Plugin-Knoten noch nicht aus. |

## Beispiel mit zwei Knoten

Für ein reales Mehrknoten-Bundle verwenden Sie eine stabile Bundle-ID wie `weather-tools` und deklarieren Sie mindestens `convert-temperature` sowie `classify-temperature`. Der erste Knoten liefert den Daten-Schlüssel `fahrenheit`; der zweite nimmt ihn später als Input und liefert `band`.

~~~json
"nodes": [
  { "id": "convert-temperature", "kind": "action", "outputs": [{ "id": "fahrenheit", "label": "Fahrenheit", "kind": "data" }], "fields": [] },
  { "id": "classify-temperature", "kind": "action", "outputs": [{ "id": "band", "label": "Band", "kind": "data" }], "fields": [{ "name": "warmAtOrAbove", "label": "Warm at or above", "kind": "number", "required": false, "secret": false }] }
]
~~~

In künftigem Go-Code prüft `Validate` die Grenzwerte vor der Ausführung. `ConvertTemperature.Execute` gibt `map[string]any{"fahrenheit": ...}` zurück; `ClassifyTemperature.Execute` liest diesen Wert und gibt `map[string]any{"band": ...}` zurück. Halten Sie Ausgabeschlüssel und Manifest-IDs synchron und kapseln Sie gemeinsame Zahl-Prüfungen in einer Hilfsfunktion. Die aktuelle v1-Runtime startet oder ruft diese Handler noch nicht auf.

Weitere Details: [Plugin-System](docs:extensions/plugin-system).
