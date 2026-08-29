# Twitch EventSub

Tragen Sie unter **Integrationen → Twitch** die öffentliche Client-ID ein und
verbinden Sie eine Identität über den Device-Code-Dialog. Tokens bleiben im
Windows-geschützten Tresor und gelangen nie in den Renderer.

Der **Twitch Event Trigger** wählt Event, Kanalname und Autorisierungsidentität.
Geben Sie den Twitch-Login ein (zum Beispiel `your_channel`), nicht die
numerische Benutzer-ID; Neuropipe löst sie lokal auf.
Für Chatnachrichten stehen Text, Command text, Broadcaster ID, Author ID und
Message ID zur Verfügung. Ein Präfix wird standardmäßig ohne Beachtung der
Groß-/Kleinschreibung geprüft; eine Autorenliste und Präfix müssen beide passen.

Verbinden Sie bei einer Antwort Message ID mit Reply to message ID von **Send
Twitch Chat Message**. Das Netzwerkrecht ist erforderlich; Nachrichten über
500 Zeichen werden abgelehnt und nicht aufgeteilt.

Für einen nach Autor gefilterten Befehl setzen Sie etwa das Präfix `!mod` und
tragen die erlaubten numerischen Autoren-IDs in **Author IDs** ein. Präfix und
Autorenfilter müssen beide passen; Text bleibt unverändert.
