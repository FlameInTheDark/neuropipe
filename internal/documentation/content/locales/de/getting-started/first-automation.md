# Ihre erste Automatisierung

Dieses Beispiel sendet eine Desktop-Benachrichtigung über eine Schaltfläche.

## Erstellen

1. Erstellen Sie eine Pipeline und fügen Sie **Button Trigger** hinzu.
2. Ziehen Sie vom **Start**-Exec-Pin zu **Desktop Notification**.
3. Legen Sie im Inspektor Titel und Nachricht fest.
4. Klicken Sie zum Testen des Entwurfs auf **Ausführen**. Das Ausführungsprotokoll zeigt Eingaben, Ausgaben und Fehler jedes Knotens.
5. Veröffentlichen Sie den gültigen Graphen. Ihr Button erscheint dann im Trigger-Board.

## Daten bewusst verbinden

Um einen berechneten Wert in die Nachricht einzufügen, verbinden Sie eine reine Ausgabe von **Format Text** oder **Get Field** mit dem Daten-Pin Nachricht. Die Benachrichtigung wird weiterhin nur ausgeführt, nachdem ihr Exec-Eingang ausgelöst wurde.

```text
Button Trigger ──exec──> Desktop Notification
Format Text ──data──> Desktop Notification.Message
```

Lesen Sie [Desktop Notification](docs:reference/local) für Berechtigungen und Fehlerverhalten.
