# HTML extrahieren

## Zweck
Extrahiert exakte Werte aus einem HTML-Dokument mit CSS-Selektoren – wie der HTML-Node in n8n. Jede konfigurierte Abfrage erzeugt einen eigenen typisierten Ausgabe-Pin.

## Konfiguration

Verbinden Sie ein HTML-Dokument mit dem Eingang **HTML** und fügen Sie dann Extraktionen im Inspektor hinzu. Jede Extraktion legt fest:

- **Pin-Name** für das Ausgabekabel.
- **CSS-Selektor** für übereinstimmende Elemente, zum Beispiel `h1.title` oder `ul li a`.
- **Rückgabewert**: **Text** (Textinhalt des Elements), **HTML** (das gerenderte Markup des Elements) oder **Attribut** (ein Attribut wie `href`; das Feld für den Attributnamen erscheint bei Auswahl).
- **Alle Treffer zurückgeben**: Aus liefert den ersten Treffer als Text; An liefert jeden Treffer als Liste von Textwerten.

Ausgaben nutzen ausschließlich die Standarddatentypen (Text oder Liste) und verbinden sich daher direkt mit Text formatieren, Für-jedes, Verbinden, Regex und allen anderen Nodes.

Ein Selektor ohne Treffer ist kein Fehler: Die Ausgabe ist leerer Text, beziehungsweise eine leere Liste bei „Alle Treffer zurückgeben“. Ein ungültiger CSS-Selektor oder ein doppelter Pin-Name schlägt vor der Ausführung der Pipeline bei der Validierung fehl.

## Beispiel
`HTTP-Anfrage → HTML extrahieren (Selektor `a.product`, Rückgabewert: Attribut `href`, Alle Treffer) → Für-jedes`.

Kombinieren Sie dies mit den HTTP-Anfrage-Schaltern **Skripte entfernen** und **Styles entfernen**, um dem Extrakteure sauberes Markup zu übergeben.
