# Textknoten

Textknoten arbeiten nur mit exakten Textwerten ohne implizite Konvertierung.
**Split** liefert `list[string]` und erhält leere Segmente. **Join** akzeptiert
nur `list[string]`. **Contains**, **Starts With** und **Ends With** vergleichen
groß-/kleinschreibungssensitiv. **Replace** ersetzt den ersten Treffer, eine
positive Anzahl oder alle Treffer; leerer Suchtext schlägt explizit fehl.

**Index Of** und **Substring** verwenden Unicode-Codepoint-Offsets. Nicht
vorhandene Werte haben den Index `-1`; ungültige Bereiche schlagen fehl.
