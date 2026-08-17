# Aus Web herunterladen

## Zweck
Lädt eine Datei von einer URL herunter und speichert sie in einem lokalen Verzeichnis. Der Dateiname wird aus dem letzten Pfadsegment der URL abgeleitet.

## Konfiguration
- **URL**: vollständige HTTP(S)-URL der herunterzuladenden Datei. Die URL muss ein Pfadsegment enthalten, das als lokaler Dateiname verwendet wird.
- **Speicherort**: absoluter Pfad zum Zielverzeichnis. Das Verzeichnis wird bei Bedarf erstellt.

## Beispiel
`Schaltflächen-Auslöser → Aus Web herunterladen (URL: https://example.com/report.pdf, Speicherort: C:\\Downloads) → Desktop-Benachrichtigung`. Verbinden Sie `Konstante` (Text) mit dem **URL**-Pin, um eine zur Laufzeit gewählte URL herunterzuladen.
