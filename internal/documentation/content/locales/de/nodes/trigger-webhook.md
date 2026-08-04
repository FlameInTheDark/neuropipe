# Local Webhook

Der Local-Webhook-Event startet eine vertrauenswürdige veröffentlichte Pipeline,
wenn Neuropipe eine korrekt HMAC-signierte HTTP-Anfrage empfängt. Aktivieren Sie
zuerst den API-Listener unter **Einstellungen → API & Webhooks**.

## Pins

- **Start** ist der Exec-Ausgang und führt zum ersten Aktions- oder Flow-Knoten.
- Der Datenausgang enthält das Anfrageobjekt. Verwenden Sie **Get Field**, um
  Werte wie json.event, json.repository oder body typisiert zu lesen.

## Konfiguration

| Feld | Erforderlich | Beschreibung |
| --- | --- | --- |
| **Path** | Ja | Verwenden Sie einen eindeutigen Pfad wie /build-complete. Äußere Schrägstriche werden normalisiert. |
| **Signing secret** | Ja | Im Windows-geschützten Neuropipe-Vault ausgewähltes Secret. Es erscheint nie im Canvas oder im Run Log. |

Die Zustellung erfolgt per POST http://<Adresse>:<Port>/hooks/<Pfad>. Für
/build-complete am Standardlistener lautet sie
http://127.0.0.1:7878/hooks/build-complete.

## Anfrage authentifizieren

Setzen Sie X-Neuropipe-Signature auf
sha256=<hexadezimales HMAC-SHA-256 des Rohbodys>. Neuropipe signiert und
vergleicht die exakten Rohbytes zeitkonstant. Ein umformatiertes JSON, andere
Zeilenenden oder andere Kodierung lassen die Prüfung fehlschlagen. Die HMAC
authentifiziert Webhooks; der Bearer-Token gilt weiterhin nur für /v1.

## Erzeugte Werte

Bei einer gültigen Anfrage enthält der Ausgang:

- trigger: webhook;
- body: den Rohbody als Text;
- json: den geparsten Body, sofern gültiges JSON vorliegt.

Für {"event":"build.completed","build":42} können Sie mit **Get Field**
json.event als Text und json.build als Zahl anlegen.

## Ausführung und Freigabe

Webhooks sind unbeaufsichtigt. Nur eine aktivierte, veröffentlichte
Trigger-Bindung mit Vertrauen für genau diese Revision wird eingereiht.
Veröffentlichte Änderungen können neues Vertrauen erfordern. Eine erfolgreiche
Anfrage gibt **202 Accepted** samt Execution-Datensatz zurück; die Pipeline kann
danach noch laufen oder fehlschlagen.

## Fehler

Deaktivierte API, unbekannte oder deaktivierte Pfade, fehlende Secrets,
ungültige Signaturen und nicht vertrauenswürdige Revisionen starten keine
Pipeline. Ein gültiger HMAC schützt nicht vor nachfolgenden Fehlern wie
abgelehnten Fähigkeiten oder Providerfehlern; diese stehen redigiert im Run Log.

## Beispiel

~~~text
Local Webhook → Get Field (json.repository, json.status)
              → Branch (status == "success") → Create Report
~~~

Vollständige Einrichtung, Signierbeispiele und Troubleshooting finden Sie unter
[API und Webhooks](docs:concepts/api-webhooks).
