# Ajouter au tableau

## Objectif

Ajoute une valeur à une liste et produit la nouvelle liste. Le nœud est pur : il ne modifie jamais le tableau connecté, donc la même liste reste réutilisable ailleurs.

## Entrées

- **Tableau** : la liste à étendre.
- **Valeur** : l’élément à ajouter.

## Sortie

- **Tableau** : la nouvelle liste avec l’élément ajouté.

## Configuration et exemple

Connectez **Tableau** à une sortie de `Analyser JSON` et **Valeur** à un nœud Constante. Plusieurs nœuds « Ajouter au tableau » enchaînés construisent une liste élément par élément. Si **Tableau** n’est pas une liste, le chemin d’exécution demandeur échoue.