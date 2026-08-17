# Afficher un dialogue de saisie

## Objectif
Affiche une boîte de dialogue stylisée avec un titre, un message, un champ de saisie étiqueté et des boutons Continuer/Annuler. L'exécution est bloquée jusqu'à la réponse de l'utilisateur. Continuer émet la valeur typée sur la broche Valeur et suit la broche Continuer ; Annuler suit la broche Annulé et émet nil sur la broche Valeur.

## Configuration
- **Titre** : texte affiché dans la barre de titre du dialogue.
- **Message** : texte affiché dans le corps du dialogue, généralement une invite pour la saisie attendue.
- **Étiquette du champ** : étiquette affichée à côté du champ de saisie.
- **Type de saisie** : `text` accepte toute chaîne, `number` analyse la saisie comme un flottant et échoue si elle est invalide. La broche de sortie Valeur suit ce type.

## Exemple
`Déclencheur de bouton → Afficher un dialogue de saisie (number) → Continuer → Mathématiques : Additionner (utiliser la broche Valeur), Annulé → Notification de bureau`. La broche Valeur est `nil` lorsque l'utilisateur annule.
