# Lister un répertoire

## Objectif

Liste les fichiers, dossiers et liens symboliques d’un répertoire local
approuvé. Chaque entrée contient un nom, un chemin absolu, une taille en
octets, un type, la date de création lorsque le système la fournit, et la date
de mise à jour.

## Exemple

`Déclencheur bouton → Lister un répertoire → Boucle pour chaque → Assertion de type`.

La broche **Fichiers** est une liste typée d’enregistrements. Connectez-la à
une boucle générique, ou utilisez **Assertion de type** avant un nœud qui
exige la structure exacte de l’entrée.
