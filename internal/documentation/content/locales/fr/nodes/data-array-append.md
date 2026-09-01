# Ajouter au tableau

## Objectif

Ajoute à une liste et produit la nouvelle liste. Le nœud est pur : il ne modifie jamais le tableau connecté, donc la même liste reste réutilisable ailleurs.

Le mode **Ajout** décide ce que signifie l’entrée Valeur : **Élément unique** ajoute la valeur comme un seul élément — une liste connectée ici s’imbrique comme un élément unique. **Éléments de tableau** concatène : les éléments de la liste connectée sont ajoutés un à un, si bien que `[3, 4]` ajouté à `[1, 2]` produit `[1, 2, 3, 4]`.

## Entrées

- **Tableau** : la liste à étendre.
- **Valeur** : l’élément ou la liste à ajouter.

## Sortie

- **Tableau** : la nouvelle liste avec l’ajout.

## Configuration et exemple

Connectez **Tableau** à une sortie de `Analyser JSON` et **Valeur** à un nœud Constante. Pour fusionner deux listes, choisissez **Éléments de tableau** et connectez **Valeur** à la seconde liste. Plusieurs nœuds « Ajouter au tableau » enchaînés construisent une liste élément par élément. Si **Tableau** n’est pas une liste, le chemin d’exécution demandeur échoue ; en mode **Éléments de tableau**, **Valeur** doit aussi être une liste.
