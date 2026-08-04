# Anbieter und Modelle

Neuropipe verwendet jeweils einen aktiven Anbieter. Wählen Sie ihn unter **Einstellungen → Anbieter**.

## Unterstützte Modi

- **Ollama** verbindet sich mit einem bereits laufenden lokalen Ollama-Endpunkt.
- **Verwaltetes llama.cpp** lädt eine von Neuropipe verwaltete Laufzeit und ein GGUF-Modell herunter und bindet sie nur an Loopback.
- **OpenAI-kompatibel** verbindet sich mit einem vom Benutzer konfigurierten kompatiblen Endpunkt.

## Lokales Modell einrichten

Suchen Sie unter **Einstellungen → Modelle** öffentliche GGUF-Repositories, wählen Sie eine Quantisierung und installieren Sie sie im konfigurierten Inhaltsordner. Die Installation prüft LFS-SHA-256, wenn der Hub es bereitstellt, und speichert lokale Metadaten neben dem Modell. Wählen Sie das installierte Modell unter **Einstellungen → Laufzeit** und starten Sie dann die verwaltete Laufzeit.

Die LLM-Warteschlange begrenzt parallele Modellarbeit. Stellen Sie sie auf eins, wenn die lokale Laufzeit keine parallelen Anfragen bedienen kann.

Anbieter-Zugangsdaten bleiben im von Windows geschützten Tresor. Knotenvorschauen, Protokolle, Exporte und Plug-in-Diagnosen schwärzen aufgelöste Geheimnisse.
