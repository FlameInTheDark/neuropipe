# Afficher un message

## Objectif
Affiche une fenêtre de dialogue native avec un titre, un message et un bouton OK. L'exécution est bloquée jusqu'à ce que l'utilisateur ferme la boîte de dialogue, puis reprend depuis la broche Puis.

## Configuration
- **Titre** : texte affiché dans la barre de titre du dialogue.
- **Message** : texte affiché dans le corps du dialogue. Le texte multiligne est pris en charge.

## Exemple
`Déclencheur de bouton → Afficher un message (Titre : Terminé, Message : Pipeline terminé)`. Connectez `Formater le texte` à la broche **Message** pour afficher des valeurs dynamiques.
