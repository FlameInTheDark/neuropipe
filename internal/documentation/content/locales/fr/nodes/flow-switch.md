# Switch

## Objectif

Switch est un nœud de contrôle Blueprint impur. Une impulsion Exec résout la
broche de données **Valeur**, teste les cas configurés dans l’ordre et suit
exactement une sortie Exec : le premier cas correspondant ou **Par défaut**.

## Broches et configuration

- **Exec** démarre la comparaison ; **Valeur** accepte toute donnée.
- Chaque cas possède une valeur littérale typée et un **nom de broche** visible.
  Son identifiant interne reste stable lors du renommage.
- Les cas sont testés de haut en bas. Le premier résultat vrai gagne ;
  **Par défaut** est exécuté lorsqu’aucun cas ne correspond.

Égal et différent acceptent texte, nombre et booléen. Contient, Commence par et
Se termine par exigent du texte. Les variantes Supérieur/Inférieur exigent des
nombres. Neuropipe n’effectue aucune conversion implicite : `"5"` n’est pas `5`.

## Exemple

Reliez `Get Field priority` à **Valeur**. Avec `Contient`, le cas `urgent`
envoie une notification urgente, `review` crée un rapport et **Par défaut**
continue le flux normal.
