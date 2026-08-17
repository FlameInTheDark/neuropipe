# Daten-Umleitung

## Zweck
Organisiert einen Daten Draht um, ohne seinen Wert zu ändern. Die Umleitung verhält sich wie ein Pin an einem Knoten: Ihr Eingang akzeptiert genau einen Draht, ihr Ausgang kann mehrere Ziele versorgen.

## Typisiertes Durchreichen
Der Ausgabe-Pin spiegelt, was den Eingang speist: Verbinden Sie einen Text-Draht, wird der Ausgang zu Text; verbinden Sie einen Zahl-Draht, wird er zu Zahl – Pin-Farbe, Typprüfung und die Farben folgender Drähte folgen stets der verbundenen Quelle. Wird der Eingang getrennt, kehrt der Pin zu Beliebig zurück, bis ein neuer Draht anliegt.

Einfügen über das Kontextmenü des Drahts (**Umleitung einfügen**) oder aus der Palette; ziehen Sie vom Ausgabe-Pin, um weitere Verbindungen zu erstellen.

## Beispiel
`LLM-Prompt.Result → Daten-Umleitung → Bericht erstellen`, mit einem zweiten Draht von derselben Umleitung zu Feld abrufen.
