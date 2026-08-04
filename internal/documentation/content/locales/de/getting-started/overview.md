# Neuropipe – Überblick

Neuropipe ist ein lokaler Automatisierungsarbeitsbereich für Windows. Eine Pipeline ist ein Blueprint-Graph: Weiße **Exec**-Verbindungen bestimmen, was ausgeführt wird, während farbige **Daten**-Verbindungen Werte nur liefern, wenn ein Knoten sie benötigt.

## Der Arbeitsbereich

- **Auslöser** zeigt veröffentlichte Button-Pipelines und ihre Tastenkürzel.
- **Pipelines** enthält Entwürfe und veröffentlichte Automatisierungen.
- **Funktionen** enthält wiederverwendbare Blueprint-Graphen.
- **Berichte** speichert lokale Markdown-Ausgaben von Berichtsknoten.
- **Chat** spricht mit einem Modell oder einer per Chat ausgelösten Pipeline.
- **Einstellungen** verwaltet einen aktiven Anbieter, lokale Modelle, Berechtigungen, Plug-ins und die optionale API.

## Sicherer Ablauf

Erstellen und testen Sie einen Entwurf und veröffentlichen Sie dann eine validierte Revision. Veröffentlichungen mit geänderten Berechtigungen benötigen erneut Vertrauen, bevor unbeaufsichtigte Zeitpläne und Webhooks laufen können. Ausführungsdaten bleiben lokal und werden vor der Aufbewahrung bereinigt.

Weiter mit [Ihre erste Automatisierung](docs:getting-started/first-automation) oder [Blueprint-Exec- und Daten-Pins](docs:concepts/blueprint-exec-data) lesen.
