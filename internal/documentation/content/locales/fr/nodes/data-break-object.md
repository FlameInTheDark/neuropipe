# Décomposer un objet

Sépare un objet en broches de sortie typées et configurables.

## Configuration

Connectez un objet à **Source**. Chaque sortie possède un ID stable, un nom,
un chemin pointé et un type. `client.nom` lit une valeur imbriquée et
`items.0.nom` peut lire un élément de liste.

Pour les résultats connus des nœuds internes, **Configurer automatiquement**
crée tous les champs documentés. Les mappages restent modifiables ensuite.

## Exemple

Résultat de terminal → Décomposer un objet (`terminal.output`) → invite LLM.
