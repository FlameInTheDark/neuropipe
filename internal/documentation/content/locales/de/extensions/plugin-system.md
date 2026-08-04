# Plugin-System

Neuropipe-Plugins sind lokale, unabhängig versionierte Bundles. Ein Bundle
enthält plugin.json, einen Sidecar-Prozess, optionale Knotendeklarationen und
optionale Markdown-Dokumentation. Der Plugin-Ordner wird unter
**Einstellungen → Erweiterungen** gewählt; anschließend werden Bundles bewusst
neu erkannt. Der Desktop-Renderer liest nie direkt aus einem Plugin-Ordner.

## Aktuelle v1-Grenze

Die aktuelle Neuropipe-Implementierung bietet **Erkennung, Diagnosen und
Dokumentationsladen**:

- Sie sucht rekursiv nach plugin.json.
- Sie prüft Identität, API-Version und die deklarierte Sidecar-Datei.
- Sie zeigt Name, Version, Beschreibung, deklarierte Knotenanzahl und
  Gesundheitsstatus in den Einstellungen.
- Sie lädt optionale Markdown-Seiten sicher in die Dokumentation.

Der Host startet noch keinen Sidecar, registriert keine Manifest-Knoten in der
Bibliothek und führt keine Plugin-Aktion aus. Der Status **Healthy** bestätigt
nur die erfolgreiche Erkennung, keinen gestarteten Prozess oder Selbsttest.

Ein Dokumentations-/Erkennungs-Bundle kann heute funktionieren. Deklarierte
Action-, Trigger- oder Tool-Knoten dürfen erst verwendet werden, wenn die unten
beschriebene Emerald-kompatible Runtime implementiert ist.

## Bundle-Struktur

~~~text
<Plugin-Ordner>/
  acme-status/
    plugin.json
    sidecar.exe
    docs/
      status.md
~~~

Relative Executable-Pfade werden vom Ordner mit plugin.json aufgelöst. Das macht
vollständige Bundles transportabel. Absolute Pfade akzeptiert der aktuelle
Validator zwar, sie sind für verteilte Bundles jedoch ungeeignet.

## Manifest

| Feld | Erforderlich | Bedeutung |
| --- | --- | --- |
| id | Ja | Stabile eindeutige Plugin-ID. |
| name | Ja | Anzeigename in den Einstellungen. |
| apiVersion | Ja | Muss exakt v1 sein. |
| executable | Ja | Vorhandene Datei, kein Ordner. |

Version und Beschreibung werden empfohlen. Ein minimales Manifest:

~~~json
{
  "id": "acme-status",
  "name": "Acme Status",
  "version": "0.1.0",
  "description": "Lokale Statusprüfungen.",
  "apiVersion": "v1",
  "executable": "sidecar.exe",
  "nodes": [],
  "documentation": []
}
~~~

Args werden gelesen, aber noch nicht ausgeführt. Speichern Sie niemals Secrets,
Token, private URLs oder Kundendaten im Manifest.

## Knotendeklarationen

pkg/pluginapi enthält Bundle und NodeSpec. Eine Deklaration enthält id, kind,
Label, Beschreibung, Icon, Farbe, Capabilities, Output-Metadaten und
Inspector-Felder. Derzeit erhöhen diese Einträge nur die angezeigte
Knotenanzahl. Sie erzeugen keinen Bibliotheksknoten und validieren keine
Konfiguration. Halten Sie IDs trotzdem stabil.

Das Paket besitzt außerdem ein Go-Action-Interface mit Validate und Execute.
Es ist kein Interprozess-Protokoll: Ein separat kompiliertes Programm wird
dadurch nicht automatisch ausführbar, weil Neuropipe noch keinen RPC-Client
oder Sidecar-Lebenszyklus besitzt.

## Emerald-kompatible Runtime

Emerald verwendet Go-First-Sidecars mit HashiCorp go-plugin und gRPC. Neuropipe
sollte dieses Modell übernehmen:

1. Einen verwalteten langlebigen Sidecar je vertrauenswürdigem Bundle starten.
2. Einen festen go-plugin-Handshake durchführen und ausschließlich gRPC
   zulassen.
3. Describe aufrufen und Laufzeit-ID sowie API-Version mit dem Manifest prüfen.
4. Verifizierte Knoten in die gemeinsame Blueprint-Bibliothek und
   Kontextsuche registrieren.
5. ValidateConfig vor Veröffentlichung/Ausführung und ExecuteAction mit
   Cancellation verwenden.
6. ToolDefinition und ExecuteTool für Agent-Tools anbieten.
7. Einen bidirektionalen TriggerRuntime-Stream je Trigger-Plugin mit vollständigen
   Subscription-Snapshots verwenden.
8. Sidecars bei Reload, Deaktivierung, Fehler und App-Shutdown zuverlässig
   stoppen.

Dieser Transport und das öffentliche Neuropipe-SDK existieren noch nicht.
Dieser Abschnitt beschreibt die Zielkompatibilität, keine heute aufrufbare API.
Die spätere Runtime muss Blueprint-Pins, Cache-Grenzen, Capabilities,
Freigaben, Trust, Cancellation, Budget, Redaction und Metriken über die
RPC-Grenze tragen.

## Plugin-Dokumentation

Ein Bundle kann lokale Markdown-Seiten beitragen. Jeder Eintrag benötigt eine
stabile id, title, categoryPath und einen relativen Markdown-Pfad; summary und
nodeTypes sind optional. Die sichtbare Dokument-ID lautet
plugin:<plugin-id>:<document-id>.

Der Pfad muss relativ sein, auf .md enden, innerhalb des Bundles bleiben und
auf eine vorhandene Datei von höchstens 1 MiB zeigen. IDs müssen im Bundle
eindeutig sein. Ein Dokumentationsfehler deaktiviert das Bundle nicht; die
Einstellungen zeigen eine eigene Diagnose. Markdown wird sicher mit dem
gemeinsamen Renderer verarbeitet.

## Gesundheit und Updates

Nach jeder Änderung auf **Plugins neu erkennen** klicken. Das ist keine
Vertrauensfreigabe und startet in der aktuellen Version keinen Sidecar. Prüfen
Sie Herkunft und Quellcode jedes lokal installierten Plugins.

- **Healthy**: id, name, v1 und Sidecar-Datei haben die Erkennung bestanden.
- **Invalid manifest**: JSON, Pflichtfeld oder API-Version korrigieren.
- **Sidecar unavailable**: Datei bauen oder den Pfad korrigieren.
- **Dokumentationsdiagnose**: Metadaten, Pfad oder Größenlimit korrigieren.

Halten Sie Plugin-, Knoten-, Output- und Dokument-IDs stabil und verwenden Sie
semantische Versionen.

Weiter mit [Ihrem ersten Plugin](docs:extensions/first-plugin).
