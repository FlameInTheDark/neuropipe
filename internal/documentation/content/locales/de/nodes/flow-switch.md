# Switch

## Zweck

Switch ist ein unsauberer Blueprint-Steuerknoten. Ein Exec-Impuls löst den
Datenpin **Wert** auf, prüft die konfigurierten Fälle der Reihe nach und folgt
genau einem Exec-Ausgang: dem ersten passenden Fall oder **Standard**.

## Pins und Konfiguration

- **Exec** startet den Vergleich; **Wert** akzeptiert jeden Datentyp.
- Jeder Fall besitzt einen typisierten Literalwert und einen sichtbaren
  **Pin-Namen**. Die interne Pin-ID bleibt beim Umbenennen stabil.
- Fälle werden von oben nach unten geprüft. Der erste Treffer gewinnt;
  **Standard** läuft, wenn kein Fall passt.

Gleich und Ungleich unterstützen Text, Zahlen und Boolean. Enthält, Beginnt
mit und Endet mit brauchen Text. Größer/Kleiner-Varianten brauchen Zahlen.
Neuropipe führt keine impliziten Umwandlungen durch: `"5"` ist nicht `5`.

## Beispiel

Verbinden Sie `Get Field priority` mit **Wert**. Mit dem Vergleich `Enthält`
führt der Fall `urgent` zu einer dringenden Benachrichtigung, `review` zu einem
Bericht und **Standard** zum normalen Ablauf.
