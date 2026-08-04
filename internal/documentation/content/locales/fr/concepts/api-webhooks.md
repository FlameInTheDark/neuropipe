# API et webhooks

L'API HTTP de Neuropipe est facultative et désactivée par défaut. Tant qu'elle
n'est pas activée dans **Paramètres → API et webhooks**, l'application n'ouvre
aucun écouteur HTTP et les déclencheurs Local Webhook ne peuvent pas recevoir
de requêtes.

Cette page décrit la configuration de l'API et l'envoi sécurisé d'un webhook
signé. Pour les broches et le comportement dans le graphe, consultez la
[référence du nœud Local Webhook](docs:node:trigger:webhook).

## Conditions nécessaires

Un webhook requiert une API active, un déclencheur Local Webhook configuré dans
une pipeline publiée, une révision publiée approuvée pour l'exécution sans
surveillance et une signature du corps brut par le secret du déclencheur.
Neuropipe ne lance pas la pipeline si l'une de ces conditions manque.

## Activer l'API locale

Ouvrez **Paramètres → API et webhooks**. Par défaut, l'API écoute
127.0.0.1:7878 et les routes normales /v1 exigent un jeton. Le jeton est
conservé uniquement dans le coffre Windows protégé. L'API d'administration est
séparée et nécessite le mode jeton.

Une adresse qui n'est pas loopback exige une confirmation explicite. Le serveur
intégré ne propose que HTTP. Pour un accès réseau, placez un proxy inverse qui
termine TLS devant Neuropipe et restreignez son accès.

Désactiver l'API arrête l'écouteur. Les nœuds et liaisons restent enregistrés,
mais leurs routes webhook deviennent indisponibles.

## Configurer le déclencheur

1. Ajoutez **Local Webhook** à un brouillon.
2. Donnez à **Path** une valeur unique, par exemple /build-complete. Les barres
   obliques externes sont normalisées.
3. Créez ou sélectionnez un **Signing secret** long et aléatoire dans le
   sélecteur de secrets.
4. Reliez **Start** au premier nœud d'action ou de flux et utilisez
   **Get Field** pour lire les données de la requête de façon typée.
5. Enregistrez et publiez la pipeline, puis activez et approuvez sa liaison de
   déclencheur.

L'URL est :

~~~text
POST http://<adresse>:<port>/hooks/<chemin>
~~~

Avec les réglages par défaut et /build-complete :
http://127.0.0.1:7878/hooks/build-complete.

## Signer le corps brut

Chaque requête doit inclure :

~~~text
X-Neuropipe-Signature: sha256=<HMAC-SHA-256 hexadécimal du corps brut>
~~~

La clé HMAC est le Signing secret. Les octets signés doivent être exactement
ceux qui sont envoyés. Reformater le JSON, modifier les fins de ligne ou
l'encodage invalide la signature. Les routes webhook utilisent ce HMAC plutôt
que le jeton bearer de l'API ; les routes /v1 restent séparées.

~~~powershell
$body = '{"event":"build.completed","build":42}'
$key = [Text.Encoding]::UTF8.GetBytes($env:NEUROPIPE_WEBHOOK_SECRET)
$bytes = [Text.Encoding]::UTF8.GetBytes($body)
$hmac = [Security.Cryptography.HMACSHA256]::new($key)
$hex = (($hmac.ComputeHash($bytes) | ForEach-Object { $_.ToString("x2") }) -join "")

Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:7878/hooks/build-complete" -ContentType "application/json" -Headers @{ "X-Neuropipe-Signature" = "sha256=$hex" } -Body $body
~~~

Une livraison valide reçoit **202 Accepted** avec un enregistrement
d'exécution. Elle est mise en file d'attente ; consultez ensuite le journal
d'exécution pour son résultat final.

## Valeurs dans le Blueprint

La sortie du déclencheur est un objet contenant trigger: webhook, body pour le
texte brut et, pour du JSON valide, json. Dans **Get Field**, définissez par
exemple json.event comme texte et json.build comme nombre. Un corps non JSON
reste valide s'il est correctement signé ; utilisez alors body.

## Sécurité et dépannage

- Conservez 127.0.0.1 sauf besoin réel d'accès distant.
- Utilisez un secret aléatoire et un chemin distinct par système émetteur.
- La publication d'une révision modifiée peut demander une nouvelle
  approbation.
- Une connexion refusée indique généralement une API désactivée, un port ou
  une adresse incorrecte.
- Une réponse 404 indique un chemin normalisé inconnu ou désactivé.
- Une signature invalide signifie que le secret, l'en-tête ou les octets bruts
  diffèrent.
- 202 confirme seulement la mise en file ; ouvrez l'exécution retournée si
  l'effet attendu n'apparaît pas.

Consultez la [référence du nœud Local Webhook](docs:node:trigger:webhook) pour
le contrat complet.
