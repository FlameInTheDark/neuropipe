# Relais de données

## Objectif
Réorganise un fil de données sans changer sa valeur. Le relais se comporte comme une broche de nœud : son entrée accepte exactement un fil, et sa sortie peut alimenter plusieurs cibles.

## Transit typé
La broche de sortie reflète ce qui alimente l'entrée : connectez un fil Texte et la sortie devient Texte ; connectez un fil Nombre et elle devient Nombre — la couleur de la broche, les contrôles de type et les couleurs des fils suivants suivent toujours la source connectée. Déconnecter l'entrée ramène la broche à Tout type jusqu'à l'arrivée d'un nouveau fil.

Insérez-en un via le menu contextuel du fil (**Insérer un relais**) ou depuis la palette ; tirez depuis sa broche de sortie pour créer chaque connexion supplémentaire.

## Exemple
`Invite LLM.Result → Relais de données → Créer un rapport`, avec un second fil du même relais vers Lire un champ.
