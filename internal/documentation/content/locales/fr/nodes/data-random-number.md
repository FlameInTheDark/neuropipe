# Nombre aléatoire

## Objectif
Génère un nombre aléatoire à chaque exécution Blueprint, avec des bornes facultatives et un choix entre nombre à virgule flottante ou entier.

## Configuration
- **Type** : `float` pour une valeur fractionnaire dans `[0, 1)` (ou votre plage), `integer` pour un nombre entier.
- **Utiliser une plage** : lorsque cette option est activée, le nœud échantillonne dans `[De, À]` au lieu de `[0, 1)`.
- **De** : borne inférieure inclusive de la plage.
- **À** : borne supérieure inclusive de la plage.

## Entrées
Les broches de données **De** et **À** sont facultatives. Lorsqu'elles sont connectées, leurs valeurs priment sur les champs de l'inspecteur, afin que les nœuds en amont contrôlent dynamiquement la plage. Lorsque l'option **Utiliser une plage** est désactivée, les bornes sont ignorées.

## Exemple
`Déclencheur de bouton → Nombre aléatoire (integer, plage 1–6) → Notification de bureau` affiche un résultat de dé. Connectez `Lire une variable` à **De** pour piloter la borne inférieure à l'exécution.
