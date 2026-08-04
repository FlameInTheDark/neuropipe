# Vue d’ensemble de Neuropipe

Neuropipe est un espace d’automatisation local pour Windows. Un pipeline est un graphe de type Blueprint : les fils blancs **exec** déterminent ce qui s’exécute, tandis que les fils **data** colorés fournissent des valeurs seulement lorsqu’un nœud en a besoin.

## L’espace de travail

- **Déclencheurs** expose les pipelines de boutons publiés et leurs raccourcis.
- **Pipelines** contient les automatisations en brouillon et publiées.
- **Fonctions** contient des graphes Blueprint réutilisables.
- **Rapports** conserve la sortie Markdown locale des nœuds de rapport.
- **Discussion** communique avec un modèle ou un pipeline déclenché par discussion.
- **Paramètres** gère un fournisseur actif, les modèles locaux, les autorisations, les plug-ins et l’API facultative.

## Un cycle de vie sûr

Créez et testez un brouillon, puis publiez une révision validée. Une publication modifiant les capacités exige à nouveau une approbation avant l’exécution de planifications et webhooks sans surveillance. Les données d’exécution restent locales et sont expurgées avant leur conservation.

Continuez avec [Votre première automatisation](docs:getting-started/first-automation) ou lisez [les broches exec et data Blueprint](docs:concepts/blueprint-exec-data).
