# Nœud JavaScript

Le nœud **JavaScript** sert aux petites transformations déterministes ou à une orchestration locale. Il utilise Goja, un interpréteur Go pur, reste synchrone et n’expose pas directement les objets Go de l’application.

## Créer une étape typée

1. Ajoutez **Code → JavaScript**, puis choisissez **Modifier le code**.
2. Ajoutez des entrées et sorties : leurs ID deviennent des variables JavaScript, et toutes les entrées sont aussi accessibles via `inputs`.
3. Choisissez le type de chaque broche avec le sélecteur visuel. Il prend en charge les types primitifs, les listes à éléments typés, les maps à clés texte et valeurs typées, ainsi que les objets structurels à champs nommés obligatoires ou facultatifs. Neuropipe enregistre le `TypeSpec` obtenu ; aucun contrat JSON n’est à modifier.
4. Retournez un objet avec toutes les sorties configurées. Chaque valeur imbriquée est validée et aucune conversion silencieuse n’est appliquée.

```js
const titles = tasks.filter((task) => !task.done).map((task) => task.title);
return { titles, count: titles.length };
```

## Lire les broches d’entrée dans le code

Chaque ID d’entrée configuré devient une variable JavaScript locale. Une entrée nommée `name` peut donc être lue avec `name` ; toutes les entrées sont aussi disponibles dans l’objet `inputs`, pratique pour une clé dynamique :

```js
const greeting = `Bonjour, ${name} !`;
const sameName = inputs.name;
const selected = inputs["name"];
return { greeting };
```

Utilisez pour chaque ID de broche un identifiant JavaScript valide, par exemple `userName`, `count` ou `filePath`. Les ID comme `first-name`, `inputs` et `np` sont refusés. Une entrée obligatoire absente arrête le nœud de façon sûre. Pour une entrée facultative, utilisez explicitement une valeur de repli, par exemple `const label = inputs.label ?? "Sans titre"`.

## API `np`

`np` offre `context`, `uuid`, `assert`, `fail`, des variables d’exécution, des helpers Base64 et SHA-256 explicites, des résumés de pipelines/fonctions/exécutions, des rapports, le chat et les notifications. Les API `np.files.list/readBytes/readText/writeBytes/writeText` demandent les droits de fichiers correspondants. `np.http.request({ url, method?, headers?, body? })` demande l’accès réseau et retourne `status`, `headers` et `body` texte.

## Capacités et limites

Chaque capacité activée est affichée dans la demande de confiance à la publication et vérifiée à l’exécution. Un script ne peut pas obtenir des droits via des imports, des chemins ou une API hôte cachée. Le code est vérifié avant enregistrement ; attente asynchrone, imports, processus, secrets et handles Go ne sont pas disponibles.
