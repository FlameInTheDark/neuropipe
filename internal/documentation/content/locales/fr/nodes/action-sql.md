# SQL

## Objectif

Execute une instruction SQL sur une base SQLite locale enregistree. Choisissez
une base, saisissez le SQL et definissez des parametres nommes. Les valeurs sont
liees via des broches d'entree typees.

## Parametres et resultats

Utilisez des noms comme `:userId` et configurez le parametre `userId`. Les
sorties sont `Columns`, `Rows`, `Rows affected`, `Last insert ID` et `Truncated`,
suivies par la sortie d'execution `Then`.

Les requetes renvoient au maximum 500 lignes. Les parametres positionnels et les
instructions multiples sont refuses.
