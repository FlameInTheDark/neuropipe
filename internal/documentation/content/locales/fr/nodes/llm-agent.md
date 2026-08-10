# Agent

## Objectif

Accomplit une tâche limitée uniquement avec des fonctions outil LLM publiées et explicitement connectées.

## Broche Outils

L’entrée **Outils** est une broche d’outil déclarative et illimitée, pas une broche Exec. Reliez-y la sortie unique **Tool** de chaque fonction outil LLM publiée. L’Agent ne présente au fournisseur que ces fonctions ; une fonction non connectée ne peut pas être appelée.

Chaque appel est contrôlé selon le nom, les types d’entrée concrets, les entrées obligatoires et les indications de la fonction publiée. Le résultat revient en JSON pour le tour de modèle suivant. Un argument invalide obtient un retour de contrat sûr afin que le modèle puisse réessayer dans sa limite de tours.

## Exemple

`Obtenir la prévision d’une ville.Tool → Agent.Outils`, puis `Déclencheur de bouton → Agent → Créer un rapport`.
