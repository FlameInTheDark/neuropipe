# Lokale Knoten

Lokale Aktionsknoten verwenden eine begrenzte Berechtigungsfreigabe. Konfigurieren Sie nur Ordner und Repositorys, auf die die veröffentlichte Revision zugreifen soll.

## Verzeichnis auflisten

**Zweck:** listet direkte Dateien, Ordner und symbolische Links in einem freigegebenen Ordner auf. **Pins:** Ausführung Ein-/Ausgang, Pfad-Eingang, Dateien-Listen-Ausgang. **Erzeugt:** `name`, `path`, Größe in Bytes, `type` (`file`, `directory` oder `symlink`), `updatedAt` und `createdAt`, wenn die Plattform diese Zeit bereitstellt. **Berechtigung:** Datei lesen. **Fehler:** nicht vorhandene, nicht lesbare oder nicht freigegebene Ordner schlagen fehl. **Beispiel:** Button-Auslöser → Verzeichnis auflisten → Für-jedes-Schleife.

## Datei lesen

**Zweck:** liest eine lokale Datei, ohne ihre Bytes zu verändern. **Pins:** Ausführung Ein-/Ausgang, Pfad-Eingang, ein Ergebnis-Ausgang. **Konfiguration:** wählen Sie Bytes oder Text für das Ergebnis. **Erzeugt:** die ausgewählte Repräsentation; Text für nicht UTF-8-konforme Inhalte schlägt sicher fehl. **Berechtigung:** Datei lesen. **Fehler:** nicht vorhandene oder nicht freigegebene Pfade schlagen fehl. **Beispiel:** Dateiüberwachung → Datei lesen → Base64 kodieren.

Mit **Base64 kodieren** und **Base64 dekodieren** wählen Sie Text- oder Byte-Array-Ein- und Ausgaben ausdrücklich. Kein lokaler Knoten wandelt diese Werte implizit um.

## Datei schreiben

**Zweck:** schreibt Text oder Rohbytes. **Pins:** Ausführung Ein-/Ausgang, Pfad-/Inhalt-Eingänge, Ergebnisobjekt. **Konfiguration:** Pfad, Inhaltstyp und Textinhalt bei Auswahl von Text. **Erzeugt:** geschriebenen Pfad und Boolean. Bytes müssen über einen verbundenen Bytes-Pin kommen und werden nie aus Text geparst. **Berechtigung:** Datei schreiben. **Fehler:** Fehler beim Elternzugriff und Schreiben werden protokolliert. **Beispiel:** Datei lesen (Bytes) → Datei schreiben (Bytes).

## Terminalbefehl ausführen

**Zweck:** führt PowerShell, Windows PowerShell oder cmd aus. **Pins:** Ausführung Ein-/Ausgang, Shell-/Befehl-/Arbeitsordner-Eingänge, Ergebnisobjekt. **Konfiguration:** Shell und Befehl. **Erzeugt:** Befehl und kombinierte Ausgabe. **Berechtigung:** Terminal. **Fehler:** Abbruch, Befehlsfehler und ungültige Arbeitsbereiche stoppen den Knoten. **Beispiel:** Button-Auslöser → Terminalbefehl ausführen → Feld abrufen `terminal.output`.

## Desktop-Benachrichtigung

**Zweck:** zeigt eine Windows-Benachrichtigung. **Pins:** Ausführung Ein-/Ausgang, Titel-/Nachricht-Eingänge, Ergebnisobjekt. **Konfiguration:** Titel und Nachricht. **Erzeugt:** angezeigten Titel/Nachricht. **Berechtigung:** keine. **Fehler:** Benachrichtigungsfehler auf nicht unterstützten Plattformen werden protokolliert. **Beispiel:** Branch True → Desktop-Benachrichtigung.

## Git

**Zweck:** führt eine gezielte Git-Operation aus. **Pins:** Ausführung Ein-/Ausgang, Operation-/Repository-Eingänge, Ergebnisobjekt. **Konfiguration:** unterstützte Operation und Repository. **Erzeugt:** Operation und Ausgabe. **Berechtigung:** Git. **Fehler:** Repository- und Befehlsfehler werden aufgezeichnet. **Beispiel:** Cron-Auslöser → Git status → Bericht erstellen.
