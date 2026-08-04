# Système de plug-ins

Les plug-ins Neuropipe sont des bundles locaux et versionnés indépendamment. Un
bundle contient plugin.json, un sidecar exécutable, des déclarations de nœuds
facultatives et de la documentation Markdown facultative. L'utilisateur choisit
la racine dans **Paramètres → Extensions**, puis demande une redécouverte. Le
renderer de bureau ne lit jamais directement un dossier de plug-in.

## Limite actuelle de v1

L'implémentation actuelle fournit **la découverte, les diagnostics et le
chargement de documentation** :

- recherche récursive de plugin.json ;
- vérification de l'identité, de la version d'API et du fichier sidecar ;
- affichage du nom, de la version, de la description, du nombre de nœuds
  déclarés et de l'état dans les paramètres ;
- chargement sécurisé de Markdown pour les bundles sains.

L'hôte ne démarre pas encore de sidecar, n'ajoute pas les nœuds déclarés à la
Bibliothèque et n'exécute pas d'action. **Healthy** signifie seulement que la
découverte a réussi, pas qu'un processus est démarré ou testé.

Un bundle de documentation/découverte peut fonctionner dès maintenant. Les
nœuds action, trigger et tool déclarés ne doivent pas être utilisés avant
l'implémentation du runtime compatible Emerald ci-dessous.

## Structure du bundle

~~~text
<racine-des-plug-ins>/
  acme-status/
    plugin.json
    sidecar.exe
    docs/
      status.md
~~~

Les chemins relatifs d'exécutable sont résolus depuis le dossier de plugin.json.
Un chemin absolu est accepté aujourd'hui, mais il ne convient pas à la
distribution d'un bundle portable.

## Manifest

| Champ | Requis | Rôle |
| --- | --- | --- |
| id | Oui | Identifiant stable et unique du plug-in. |
| name | Oui | Nom affiché dans les paramètres. |
| apiVersion | Oui | Doit être exactement v1. |
| executable | Oui | Fichier existant qui n'est pas un dossier. |

Version et description sont recommandées. Exemple :

~~~json
{
  "id": "acme-status",
  "name": "Acme Status",
  "version": "0.1.0",
  "description": "Vérifications locales d'état.",
  "apiVersion": "v1",
  "executable": "sidecar.exe",
  "nodes": [],
  "documentation": []
}
~~~

Les args sont lus mais l'exécutable n'est pas démarré. Ne placez jamais de
secrets, jetons, URL privées ou données client dans ce JSON.

## Déclarations de nœuds

pkg/pluginapi fournit Bundle et NodeSpec. Une déclaration comprend id, kind,
libellé, description, icône, couleur, capacités, sorties et champs de
configuration. Aujourd'hui, ces entrées ne font qu'augmenter le nombre de
nœuds affiché. Elles ne créent pas de nœud dans la Bibliothèque et ne valident
pas de configuration. Conservez tout de même les ids stables.

Le paquet expose aussi une interface Go Action avec Validate et Execute. Ce
n'est pas un protocole interprocessus : un exécutable séparé ne peut pas être
exécuté simplement en déclarant cette interface, car Neuropipe ne possède pas
encore de client RPC ni de cycle de vie sidecar.

## Runtime compatible Emerald

Emerald utilise des sidecars Go-first avec HashiCorp go-plugin et gRPC.
Neuropipe doit adopter le même modèle :

1. démarrer un sidecar long-vivant et géré par bundle approuvé ;
2. effectuer le handshake go-plugin et n'autoriser que gRPC ;
3. appeler Describe et vérifier id et version d'API face au manifest ;
4. enregistrer les nœuds vérifiés dans la Bibliothèque Blueprint ;
5. utiliser ValidateConfig et ExecuteAction avec annulation ;
6. proposer ToolDefinition et ExecuteTool pour les outils d'agent ;
7. maintenir un flux TriggerRuntime bidirectionnel avec snapshots complets ;
8. arrêter les sidecars lors d'un rechargement, d'une désactivation, d'une
   erreur ou de l'arrêt de l'application.

Le transport et le SDK public Neuropipe n'existent pas encore. Cette section
décrit une direction de compatibilité, pas une API disponible. Le runtime devra
porter les pins Blueprint, les limites de cache, capacités, approbations,
confiance, annulation, budgets, masquage et métriques à travers RPC.

## Documentation de plug-in

Chaque page comprend id, title, categoryPath et un chemin Markdown relatif ;
summary et nodeTypes sont facultatifs. L'ID visible devient
plugin:<plugin-id>:<document-id>.

Le chemin doit rester dans le bundle, se terminer par .md, référencer un
fichier existant de 1 MiB maximum et utiliser un id unique dans le bundle. Une
erreur de documentation ne désactive pas le bundle : les paramètres affichent
un diagnostic séparé. Markdown est traité par le renderer sûr commun.

## Santé et mises à jour

Après chaque changement, cliquez **Redécouvrir les plug-ins**. Ce n'est pas une
autorisation de confiance et aucun sidecar n'est démarré dans cette version.
Vérifiez la provenance et le code de tout plug-in local.

- **Healthy** : id, name, v1 et fichier sidecar sont valides.
- **Invalid manifest** : corrigez JSON, champ requis ou version.
- **Sidecar unavailable** : construisez le fichier ou corrigez son chemin.
- **Diagnostic de documentation** : corrigez métadonnées, chemin ou taille.

Conservez les ids et utilisez des versions sémantiques. Passez à [votre premier
plug-in](docs:extensions/first-plugin).
