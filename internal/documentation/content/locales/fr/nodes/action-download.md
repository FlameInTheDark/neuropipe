# Télécharger depuis le Web

## Objectif
Télécharge un fichier depuis une URL et l'enregistre dans un dossier local. Le nom du fichier est déduit du dernier segment de chemin de l'URL.

## Configuration
- **URL** : URL HTTP(S) complète du fichier à télécharger. L'URL doit inclure un segment de chemin qui devient le nom du fichier local.
- **Emplacement** : chemin absolu vers le dossier de destination. Le dossier est créé s'il n'existe pas.

## Exemple
`Déclencheur de bouton → Télécharger depuis le Web (URL : https://example.com/report.pdf, Emplacement : C:\\Downloads) → Notification de bureau`. Connectez `Constante` (texte) à la broche **URL** pour télécharger une URL choisie à l'exécution.
