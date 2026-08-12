# Définir une variable globale

## Objectif
Écrit une variable de portée espace de travail, partagée entre toutes les
pipelines et toutes les exécutions. La valeur est immédiatement disponible en
mémoire et persistée dans la base de données locale au plus une fois par
seconde, donc elle survit au redémarrage de l'application.

Le nœud effectue l'une des trois opérations, choisie dans l'inspecteur :

- **Définir** écrase la valeur, en la validant contre le type de données
  déclaré.
- **Incrémenter** ajoute atomiquement un nombre, ce qui évite les mises à jour
  perdues lorsque deux pipelines s'exécutent en parallèle.
- **Ajouter** ajoute atomiquement un élément à une liste.

## Exemple
`Déclencheur planifié → Définir une variable globale (lastRun, opération :
Définir)` pour enregistrer la dernière exécution d'une pipeline de maintenance.
