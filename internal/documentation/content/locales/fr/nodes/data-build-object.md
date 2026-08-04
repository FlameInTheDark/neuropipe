# Construire un objet

Crée un objet typé à partir d’un nombre configurable de broches de données.
Chaque ligne de **Champs** conserve un ID stable : renommer la broche ou sa clé
n’interrompt pas les connexions existantes.

## Configuration

Définissez un nom de broche, un type et une clé d’objet pour chaque valeur. Les
chemins pointés tels que `client.nom` créent des objets imbriqués. Les clés
doivent être non vides, uniques et non chevauchantes.

## Exemple

**Nom** → `client.nom` et **E-mail** → `client.email` produisent :

~~~json
{"client":{"nom":"Ada Lovelace","email":"ada@example.com"}}
~~~
