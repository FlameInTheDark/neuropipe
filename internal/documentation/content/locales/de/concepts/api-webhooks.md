# API und Webhooks

Die HTTP-API von Neuropipe ist optional und standardmäßig deaktiviert. Solange
sie unter **Einstellungen → API & Webhooks** nicht aktiviert ist, öffnet die
App keinen HTTP-Listener und Local-Webhook-Trigger können keine Anfragen
empfangen.

Diese Seite erklärt die API-Einstellungen und die sichere Zustellung eines
signierten Webhooks. Pins und Verhalten im Graphen beschreibt die
[Local-Webhook-Knotenreferenz](docs:node:trigger:webhook).

## Voraussetzungen

Für einen funktionierenden Webhook müssen die API aktiviert sein, ein
veröffentlichter Pipeline-Entwurf einen aktivierten Local-Webhook-Trigger
enthalten, die veröffentlichte Revision für unbeaufsichtigte Ausführung
vertrauenswürdig sein und der Sender den exakten Rohtext der Anfrage mit dem
Signing Secret signieren. Fehlt eine dieser Voraussetzungen, startet
Neuropipe die Pipeline nicht.

## Lokale API aktivieren

Öffnen Sie **Einstellungen → API & Webhooks**. Standardmäßig lauscht die API
auf 127.0.0.1:7878 und verlangt für die normalen /v1-Endpunkte einen Token.
Der Token liegt ausschließlich im Windows-geschützten Vault. Der
Administrationszugang ist separat und erfordert den Token-Modus.

Eine nicht lokale Bind-Adresse benötigt eine ausdrückliche Bestätigung. Der
eingebettete Server bietet nur HTTP an. Verwenden Sie für Zugriff aus dem
Netzwerk einen TLS-terminierenden Reverse Proxy und beschränken Sie dessen
Zugriff.

Beim Deaktivieren der API stoppt der Listener. Die Knoten und Bindungen bleiben
gespeichert, ihre Webhook-Routen sind jedoch nicht erreichbar.

## Trigger konfigurieren

1. Fügen Sie im Entwurf **Local Webhook** hinzu.
2. Setzen Sie **Path** auf einen eindeutigen Pfad, etwa /build-complete.
   Führende und abschließende Schrägstriche werden normalisiert.
3. Wählen oder erstellen Sie im Secret-Picker ein langes, zufälliges
   **Signing secret**.
4. Verbinden Sie **Start** mit dem ersten Aktions- oder Flow-Knoten und
   verwenden Sie **Get Field**, um Anfragedaten typisiert zu lesen.
5. Speichern und veröffentlichen Sie die Pipeline; aktivieren und vertrauen Sie
   anschließend der Trigger-Bindung.

Der Endpunkt lautet:

~~~text
POST http://<Bind-Adresse>:<Port>/hooks/<Pfad>
~~~

Mit Standardwerten und /build-complete ist das
http://127.0.0.1:7878/hooks/build-complete.

## Rohdaten signieren

Jede Anfrage benötigt:

~~~text
X-Neuropipe-Signature: sha256=<hexadezimales HMAC-SHA-256 des Rohbodys>
~~~

Der HMAC-Schlüssel ist das Signing Secret. Signiert werden die exakten Bytes,
die auch versendet werden. JSON neu zu formatieren, Zeilenenden zu ändern oder
die Kodierung zu ändern macht die Signatur ungültig. Webhook-Routen nutzen
diese HMAC-Prüfung statt des API-Bearer-Tokens; /v1 bleibt davon getrennt.

~~~powershell
$body = '{"event":"build.completed","build":42}'
$key = [Text.Encoding]::UTF8.GetBytes($env:NEUROPIPE_WEBHOOK_SECRET)
$bytes = [Text.Encoding]::UTF8.GetBytes($body)
$hmac = [Security.Cryptography.HMACSHA256]::new($key)
$hex = (($hmac.ComputeHash($bytes) | ForEach-Object { $_.ToString("x2") }) -join "")

Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:7878/hooks/build-complete" -ContentType "application/json" -Headers @{ "X-Neuropipe-Signature" = "sha256=$hex" } -Body $body
~~~

Eine gültige Zustellung erhält **202 Accepted** mit einem Execution-Datensatz.
Die Ausführung wird eingereiht und kann später im Execution Log geprüft werden.

## Werte im Blueprint

Der Trigger gibt ein Objekt mit trigger: webhook, body als Rohtext und optional
json für gültiges JSON aus. Konfigurieren Sie in **Get Field** beispielsweise
json.event als Text und json.build als Zahl. Nicht-JSON ist bei gültiger
Signatur ebenfalls zulässig; verwenden Sie dann body.

## Sicherheit und Fehlerbehebung

- Behalten Sie 127.0.0.1, sofern kein externer Zugriff nötig ist.
- Verwenden Sie für jeden Sender einen eigenen zufälligen Secret- und Pfadwert.
- Eine Änderung der veröffentlichten Revision kann neues Vertrauen erfordern.
- Connection refused bedeutet meist: API deaktiviert, falscher Port oder
  falsche Bind-Adresse.
- 404 bedeutet: der normalisierte Pfad ist unbekannt oder nicht aktiviert.
- Signature invalid bedeutet: Secret, Header-Format oder Rohbytes stimmen nicht
  überein.
- 202 bestätigt nur die Einreihung. Prüfen Sie bei fehlender Wirkung die
  zurückgegebene Execution und den ersten fehlgeschlagenen Knoten.

Weitere Details enthält die [Local-Webhook-Knotenreferenz](docs:node:trigger:webhook).
