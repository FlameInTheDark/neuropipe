# Votre première automatisation

Cet exemple envoie une notification de bureau depuis un bouton.

## Création

1. Créez un pipeline et ajoutez **Button Trigger**.
2. Faites glisser depuis sa broche exec **Start** vers **Desktop Notification**.
3. Définissez le titre et le message de la notification dans l’inspecteur.
4. Cliquez sur **Exécuter** pour tester le brouillon. Le journal d’exécution affiche les entrées, sorties et erreurs de chaque nœud.
5. Publiez lorsque le graphe est valide. Votre bouton apparaît alors dans le panneau des déclencheurs.

## Ajouter les données délibérément

Pour placer une valeur calculée dans le message, connectez la sortie pure de **Format Text** ou **Get Field** à la broche de données Message. La notification est toujours exécutée seulement après l’impulsion de son entrée exec.

```text
Button Trigger ──exec──> Desktop Notification
Format Text ──data──> Desktop Notification.Message
```

Consultez [Desktop Notification](docs:reference/local) pour les capacités et le comportement en cas d’échec.
