# Assertion de type

## Objectif

Restreint une valeur `any` à un contrat de type Blueprint V3 explicite sans la convertir. Le nœud vérifie à l’exécution les primitives, listes, maps et champs d’enregistrement ; une incompatibilité arrête l’exécution de façon sûre.

## Exemple

`Lire un champ → Assertion de type ({"kind":"record","fields":[{"name":"id","type":{"kind":"string"}}]}) → Construire un objet`.

Utilisez **Convertir** lorsqu’une primitive doit être convertie. Utilisez **Assertion de type** uniquement lorsque la valeur satisfait déjà le contrat choisi.
