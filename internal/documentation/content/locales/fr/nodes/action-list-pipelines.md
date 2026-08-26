# Lister les pipelines

## But
Émet tous les pipelines publiés du workspace sous forme de données
structurées, afin qu''un flux puisse raisonner sur le catalogue lui-même —
filtrer, choisir une cible pour le nœud Run Pipeline, ou alimenter rapports
et notifications.

## Sorties
- **Then** : continuation exec une fois la liste collectée.
- **Pipelines** : liste d''objets avec `id`, `name`, `description`, `status`
  et `publishedRevision`. Les pipelines sans publication sont exclus.
- **Count** : nombre d''entrées de la liste.

## Configuration
Ce nœud n''a aucune configuration ; le catalogue est lu en direct à chaque
exécution.

## Exemple
`Déclencheur manuel → Lister les pipelines → JavaScript (choisir par nom) → Run Pipeline`