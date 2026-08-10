# Obtenir le type

## Objectif

Indique le type JSON de n’importe quelle valeur : `text`, `number`, `boolean`, `object`, `list` ou `null`. Pour une liste, la sortie **Type d’élément** indique le type commun des éléments.

## Sorties

- **Type** : le type JSON de la valeur.
- **Type d’élément** : pour une liste, le type commun des éléments, sinon vide.

## Configuration et exemple

Pour une liste, **Type d’élément** vaut `any` si la liste est vide, `mixed` si les éléments ne sont pas tous du même type, sinon l’un des types JSON ci-dessus.