# Lire une variable globale

## Objectif
Lit une variable de portée espace de travail, partagée entre toutes les
pipelines et toutes les exécutions. Une lecture avant toute écriture retourne
la valeur par défaut déclarée de la variable.

La variable se choisit dans une liste de noms déclarés, gérés dans l'écran
**Variables**. Les lectures sont sûres entre pipelines exécutées en parallèle.

## Exemple
`Lire une variable globale (visits) → Addition → Définir une variable globale`
pour accumuler un compteur sur plusieurs exécutions.
