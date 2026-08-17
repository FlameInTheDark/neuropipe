# Afficher une question

## Objectif
Affiche une boîte de dialogue native avec les boutons Oui et Non. L'exécution est bloquée jusqu'à ce que l'utilisateur choisisse l'une des options, puis reprend depuis la broche exec correspondante afin que le graphe puisse se ramifier selon la décision de l'utilisateur.

## Configuration
- **Titre** : texte affiché dans la barre de titre du dialogue.
- **Message** : texte affiché dans le corps du dialogue. Formulez-le comme une question oui/non.

## Exemple
`Déclencheur de bouton → Afficher une question (Message : Envoyer le rapport maintenant ?) → Oui → Requête HTTP, Non → Notification de bureau (ignorée)`. La sortie Résultat indique le bouton pressé (`yes` ou `no`).
