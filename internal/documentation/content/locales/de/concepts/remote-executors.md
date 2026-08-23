# Remote-Executors

Ein Remote-Executor ist der eigenständige Neuropipe-Pipeline-Runner, der auf einem anderen Rechner installiert wird. Er enthält die vollständige Blueprint-Engine, sodass jede Pipeline, die lokal läuft, auch dort läuft: HTTP, Terminal, Git, Dateien, JavaScript, KI-Knoten, Berichte, Chat, Twitch und mehr. Neuropipe kommuniziert über eine authentifizierte gRPC-Verbindung mit ihm.

## Was der Executor selbst hostet

Der Executor ist autonom, wo Autonomie sinnvoll ist:

- **Cron-Zeitpläne** werden auf dem Executor-Rechner ausgelöst, auch wenn Neuropipe geschlossen ist. Ein Zeitplan wird nur autonom ausgelöst, wenn seine veröffentlichte Revision beim Deployment vertraut und aktiviert war; Vertrauen wandert mit dem Deployment mit und wird bei Änderungen neu bereitgestellt.
- **Schaltflächen**, Hotkeys, Webhooks, Chat-Auslöser und Twitch-Ereignisse entstehen weiterhin in Neuropipe. Jeder Lauf einer remote-gerichteten Pipeline wird per gRPC an den Executor übergeben und erscheint im lokalen Laufverlauf.

Datei-Überwachungs-Auslöser werden als Metadaten bereitgestellt, bleiben aber derzeit – wie die anderen übergebenen Auslöser – auf dem Desktop gehostet.

## Wo Daten liegen

Ihr Rechner behält alles Wichtige:

- Pipeline-Definitionen, Revisionen, Vertrauensentscheidungen, Laufverlauf, Berichte und Unterhaltungen bleiben im lokalen Workspace.
- Im Standardmodus **Über Neuropipe** werden Modellaufrufe von Executor-Läufen über die verschlüsselte Sitzung an Ihre konfigurierten Anbieter weitergeleitet. API-Schlüssel verlassen diesen Rechner nie.
- Datenbank-Anmeldedaten und Twitch-OAuth bleiben lokal; SQL- und Twitch-Knoten auf dem Executor rufen über die Sitzung zurück.

Stellt man einen Executor auf den Modus **Auf Executor** um, werden Anbieter direkt auf dieser Maschine konfiguriert. Im Configure-Dialog eingegebene Schlüssel werden einmalig im eigenen Tresor des Executors gespeichert und können nicht ausgelesen werden.

Globale Variablen auf Executor-Seite sind pro Executor isoliert: Sie werden implizit von Pipelines erzeugt, bleiben auf dem Executor bestehen und synchronisieren sich nie mit Ihrem Workspace. Interaktive Dialogknoten schlagen auf einem Executor bewusst fehl, weil niemand antworten kann.

## Einen Executor installieren

1. Laden Sie das Archiv für die Zielplattform von der Release-Seite herunter (`neuropipe-executor-*` für Windows, Linux und macOS).
2. Starten Sie den Daemon einmal:

   ```bash
   neuropipe-executor serve
   ```

   Ohne Konfiguration erstellt er ein `data`-Verzeichnis, erzeugt einen starken gemeinsamen Token, gibt ihn **genau einmal** aus, speichert ihn unter `data/token.txt` und lauscht auf `:47777`. Kopieren Sie den angezeigten Token bei der Registrierung in Neuropipe.
3. Optional: Legen Sie eine `executor.json` neben die Binärdatei für statische Einstellungen:

   ```json
   {
     "listen": ":47777",
     "dataDir": "data",
     "tokenFile": "token.txt"
   }
   ```

   Kommandozeilen-Flags überschreiben die Datei pro Start: `neuropipe-executor serve --listen :5000 --token <wert> --data-dir D:\executor`. Die Umgebungsvariable `NEUROPIPE_EXECUTOR_TOKEN` funktioniert ebenfalls, z. B. für Dienstdefinitionen.
4. Fügen Sie den Executor in Neuropipe mit seiner Adresse hinzu (zum Beispiel `192.168.1.50:47777`) und testen Sie die Verbindung.
5. Registrieren Sie ihn unter Linux als systemd-Dienst, damit Zeitpläne Neustarts überleben.

Nützliche Befehle:

- `neuropipe-executor status` — zeigt die effektive Konfiguration, woher der Token käme (niemals seinen Wert) und wie viele Pipelines und Läufe lokal gespeichert sind.
- `neuropipe-executor token generate` — rotiert den gemeinsamen Token, speichert ihn und gibt den neuen Wert genau einmal aus; aktualisieren Sie danach Neuropipe.
- `neuropipe-executor --version`.

Transportsicherheit: Token-Authentifizierung ist immer erforderlich. Für Verkehr über nicht vertrauenswürdige Netzwerke, terminieren Sie TLS vor dem Executor (`tlsCert`/`tlsKey` in der Boot-Datei) oder erreichen Sie ihn über ein VPN und aktivieren Sie dann die Option TLS verwenden für die Registrierung.

## Pipelines für einen Executor erstellen

Verwenden Sie **Neue Pipeline** in der Pipeline-Ansicht und wählen Sie unter *Läuft auf* den Executor. Remote-gerichtete Pipelines erscheinen in der Kategorie Remote und tragen das Badge ihres Executors im Editor. Die Veröffentlichung stellt die veröffentlichte Revision – zusammen mit allen verwendeten eigenen Funktionen – automatisch bereit; ist der Executor nicht erreichbar, wird dennoch lokal veröffentlicht, und Sie können über *Mit Executor synchronisieren* oder die automatische Abstimmung erneut bereitstellen.
