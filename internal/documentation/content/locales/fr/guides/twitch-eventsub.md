# Twitch EventSub

Dans **Intégrations → Twitch**, saisissez l’ID client public puis connectez une
identité avec le dialogue de code d’appareil. Les jetons restent dans le coffre
protégé par Windows et ne vont jamais dans le renderer.

Le **Twitch Event Trigger** choisit l’événement, le nom de chaîne et l’identité
d’autorisation. Saisissez le login Twitch (par exemple `your_channel`), et non
l’identifiant numérique : Neuropipe le résout localement. Pour les messages de chat, Text, Command text, Broadcaster ID,
Author ID et Message ID sont disponibles. Le préfixe est insensible à la casse
par défaut ; le préfixe et la liste d’auteurs doivent correspondre.

Pour répondre, connectez Message ID à Reply to message ID de **Send Twitch Chat
Message**. La capacité réseau est nécessaire ; les messages de plus de 500
caractères sont rejetés sans découpage.

Pour une commande filtrée par auteur, définissez par exemple le préfixe `!mod`
et les ID numériques autorisés dans **Author IDs**. Le préfixe et l’auteur
doivent correspondre ; Text reste inchangé.
