# Exécuteurs distants

Un exécuteur distant est le runner de pipelines Neuropipe autonome installé sur une autre machine. Il contient le moteur Blueprint complet : tout pipeline qui s'exécute localement peut donc s'y exécuter — HTTP, terminal, Git, fichiers, JavaScript, nœuds d'IA, rapports, conversations, Twitch et plus encore. Neuropipe communique avec lui via une connexion gRPC authentifiée.

## Ce que l'exécuteur héberge

L'exécuteur est autonome là où cela a du sens :

- Les **planifications cron** se déclenchent sur la machine de l'exécuteur même lorsque Neuropipe est fermé. Une planification ne se déclenche de façon autonome que si sa révision publiée était approuvée et activée au moment du déploiement ; l'approbation accompagne le déploiement et est redéployée à chaque modification.
- Les **boutons**, raccourcis clavier, webhooks, déclencheurs de conversation et événements Twitch proviennent toujours de Neuropipe. Chaque exécution d'un pipeline destiné à un exécuteur distant lui est transmise via gRPC et apparaît dans votre historique local.

Les déclencheurs de surveillance de fichiers sont déployés comme métadonnées mais restent, comme les autres déclencheurs transmis, hébergés par le bureau.

## Où résident les données

Votre machine conserve tout ce qui compte :

- Définitions, révisions, décisions d'approbation, historique, rapports et conversations restent dans l'espace de travail local.
- En mode **Via Neuropipe** (par défaut), les appels IA des exécutions distantes sont relayés par la session chiffrée vers vos fournisseurs configurés. Les clés API ne quittent jamais cette machine.
- Les identifiants de bases de données et l'OAuth Twitch restent locaux ; les nœuds SQL et Twitch de l'exécuteur rappellent via la session.

En passant un exécuteur en mode **Sur l'exécuteur**, les fournisseurs sont configurés directement sur cette machine. Les clés saisies dans la boîte de dialogue Configurer sont stockées une seule fois dans le coffre de l'exécuteur et ne peuvent pas être relues.

Les variables globales côté exécuteur sont isolées par exécuteur : créées implicitement par les pipelines, elles persistent sur la machine distante et ne se synchronisent jamais avec votre espace de travail. Les nœuds de boîtes de dialogue interactives échouent explicitement sur un exécuteur, puisqu'il n'y a personne pour y répondre.

## Installer un exécuteur

1. Téléchargez l'archive de la plateforme cible depuis la page des versions (`neuropipe-executor-*` pour Windows, Linux et macOS).
2. Démarrez le démon une première fois :

   ```bash
   neuropipe-executor serve
   ```

   Sans configuration, il crée un dossier `data`, génère un jeton partagé robuste, l'affiche **une seule fois**, l'enregistre dans `data/token.txt` et écoute sur `:47777`. Copiez le jeton affiché dans Neuropipe lors de l'enregistrement.
3. Ajoutez éventuellement un `executor.json` à côté du binaire pour les réglages statiques :

   ```json
   {
     "listen": ":47777",
     "dataDir": "data",
     "tokenFile": "token.txt"
   }
   ```

   Les options en ligne de commande priment sur le fichier à chaque démarrage : `neuropipe-executor serve --listen :5000 --token <valeur> --data-dir D:\executor`. La variable d'environnement `NEUROPIPE_EXECUTOR_TOKEN` fonctionne aussi, par exemple pour une définition de service.
4. Dans Neuropipe, ajoutez l'exécuteur avec son adresse (par exemple `192.168.1.50:47777`) et testez la connexion.
5. Sous Linux, enregistrez-le comme service systemd pour que les planifications survivent aux redémarrages.

Commandes utiles :

- `neuropipe-executor status` — affiche la configuration effective, d'où viendrait le jeton (jamais sa valeur) et le nombre de pipelines et d'exécutions stockés localement.
- `neuropipe-executor token generate` — renouvelle le jeton partagé, l'enregistre et affiche la nouvelle valeur une seule fois ; mettez ensuite Neuropipe à jour.
- `neuropipe-executor --version`.

Sécurité du transport : l'authentification par jeton est toujours obligatoire. Pour du trafic sur un réseau non fiable, terminez TLS devant l'exécuteur (`tlsCert`/`tlsKey` dans le fichier de démarrage) ou joignez-le via un VPN, puis activez l'option Utiliser TLS de l'enregistrement.

## Créer des pipelines pour un exécuteur

Utilisez **Nouveau pipeline** dans la vue Pipelines et choisissez l'exécuteur sous *S'exécute sur*. Les pipelines destinés à un exécuteur apparaissent dans la catégorie Distant et portent leur badge dans l'éditeur. La publication déploie automatiquement la révision publiée — avec les fonctions personnalisées utilisées par le graphe ; si l'exécuteur est injoignable, la publication réussit localement et vous pouvez réessayer via *Synchroniser avec l'exécuteur* ou compter sur la synchronisation automatique au retour de la connexion.
