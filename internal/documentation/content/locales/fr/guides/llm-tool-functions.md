# Fonctions outil LLM

Une fonction outil LLM est une fonction Blueprint réutilisable et publiée qu’un **Agent** ou un **Agent de code** peut appeler via sa broche **Outils**. Ce n’est pas un nœud d’exécution : sa sortie Tool déclare seulement sa disponibilité au modèle. L’hôte valide l’appel, exécute la fonction et renvoie le résultat typé au modèle.

## Construire d’abord le contrat

1. Dans **Fonctions**, choisissez **Nouvelle fonction → Outil LLM**.
2. Décrivez *quand* le modèle doit l’utiliser. Exemple : « Rechercher la prévision actuelle d’une ville. À utiliser seulement pour les questions météo. »
3. Ajoutez les entrées et sorties publiques. Chaque broche doit avoir une **indication pour le modèle** précisant le sens, les contraintes et un exemple.
4. Choisissez des types concrets, rendez obligatoires seulement les entrées nécessaires, puis publiez après avoir créé un chemin atteignable **Function Entry → Function Return**.

La publication exige une description de fonction, au moins une sortie décrite, des indications sur toutes les broches, des types concrets et des noms d’arguments uniques. Un outil peut ne pas avoir d’entrée, mais il doit renvoyer un résultat décrit.

## Exemple : rechercher la météo

Créez **Obtenir la prévision d’une ville** :

| Partie | Contrat |
| --- | --- |
| Description | « Rechercher la prévision actuelle d’une ville. À utiliser pour les questions météo. » |
| Entrée `city` | Texte, obligatoire. Indication : « Ville et pays, par exemple `Yekaterinburg, RU`. » |
| Sortie `forecast` | Texte. Indication : « Prévision concise avec conditions et température. » |

Dans la fonction :

```text
Function Entry ──exec──> HTTP Request ──exec──> Function Return
      city ──data──> mappage HTTP ──data──> forecast
```

Publiez ensuite la fonction et reliez sa sortie **Tool** à l’entrée **Outils** d’un Agent. Plusieurs outils indépendants peuvent être reliés à la même broche.

Exemple d’instructions Agent :

> Réponds à la question météo. Lorsqu’une prévision actuelle est nécessaire, appelle l’outil connecté avec la ville et le pays. N’invente pas de prévision.

## Types à la frontière du modèle

Les arguments sont du JSON, puis Neuropipe les décode vers le `TypeSpec` exact avant l’exécution : texte et booléen restent des valeurs JSON, un entier ne doit pas avoir de fraction, les octets sont une chaîne Base64 décodée en `[]byte`, les listes valident chaque élément, les maps ont des clés texte et les records anonymes valident leurs champs déclarés.

`any`, les records Go nommés et les maps à clés non textuelles ne sont pas des contrats publics valides. Il n’existe aucune conversion implicite texte-vers-nombre, entier-vers-flottant ou octets-vers-texte ; utilisez un nœud de conversion explicite dans le graphe.

## Appels et sécurité

L’Agent est limité par son nombre de tours. Un argument inconnu, une valeur obligatoire absente, un type incorrect, du Base64 invalide ou un champ de record inconnu renvoie une erreur de contrat sûre au modèle afin qu’il puisse corriger l’appel. Les erreurs internes ne révèlent ni chemins locaux, ni secrets, ni données utiles.

Les capacités restent celles des nœuds internes à la fonction, et les validations locales habituelles de confiance et d’autorisation restent nécessaires. Ne placez pas de secrets dans les descriptions, indications, instructions Agent ou résultats d’outil.
