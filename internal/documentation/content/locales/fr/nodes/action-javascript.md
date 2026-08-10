# JavaScript

## Objectif

Exécute une petite action JavaScript synchrone dans l’exécution Blueprint en cours. Avec **Modifier le code**, déclarez des broches d’entrée et de sortie typées. Le programme doit retourner un objet dont les clés correspondent exactement aux sorties configurées. Aucune conversion implicite n’est faite : les valeurs doivent satisfaire le `TypeSpec` déclaré.

## Exemple

Déclarez l’entrée obligatoire `name` et la sortie `message` comme **Texte** dans le sélecteur de type, puis écrivez :

```js
return { message: `Bonjour, ${name} !` };
```

L’entrée est également disponible avec `inputs.name`.

## Lire les entrées

Les ID d’entrée deviennent des variables JavaScript locales : la broche
configurée `name` est disponible comme `name`. Chaque broche est aussi dans
l’objet `inputs`, y compris pour un accès dynamique :

```js
const direct = name;
const property = inputs.name;
const dynamic = inputs["name"];
const label = inputs.optionalLabel ?? "Sans titre";
return { message: `${direct}: ${label}` };
```

Les ID de broches doivent être des identifiants JavaScript valides comme
`userName`, `count` ou `filePath`. Les noms comme `first-name`, `inputs` et
`np` sont refusés. Une entrée obligatoire absente fait échouer le nœud de façon
sûre ; `??` est réservé aux broches facultatives.

## API système et sécurité

`np` fournit seulement des aides locales limitées pour les variables, Base64, les hachages, résumés, rapports, chat et notifications. Les fichiers et le réseau nécessitent la capacité correspondante activée dans l’éditeur. Aucun objet Go, secret ou handle hôte n’est exposé. Consultez le guide **Nœud JavaScript** pour l’API complète.
