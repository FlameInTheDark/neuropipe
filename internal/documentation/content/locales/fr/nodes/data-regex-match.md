# Correspondance regex

## Objectif

Teste du **Texte** avec une expression régulière RE2 de Go et renvoie chaque
correspondance sous forme de structure typée. Connectez Texte et définissez le
**Motif** dans l’inspecteur, ou connectez un fil Texte au motif pour remplacer
la valeur enregistrée durant cette exécution.

## Sorties

- **Correspondance trouvée** est vrai si au moins une correspondance existe.
- **Nombre** est le nombre entier exact de correspondances.
- **Correspondances** est `list[RegexMatch]`, avec `text`, `startByte`,
  `endByte` et `captures`.

Chaque groupe capturé est un `RegexCapture` avec un `index` commençant à un,
son `name` (vide pour un groupe non nommé), `matched`, `text`, `startByte` et
`endByte`. Un groupe optionnel absent a des valeurs sûres : `matched=false`,
texte vide et offsets `-1`. Les offsets sont des positions d’octets UTF-8,
comme dans le paquet Go `regexp`.

L’absence de correspondance n’est pas une erreur : Correspondance trouvée est
false, Nombre vaut zéro et Correspondances est une liste vide.

## Exemple

Utilisez le motif `(?P<name>\w+)=(?P<value>\d+)` avec le texte
`limit=25 retries=3`. Correspondances contient deux enregistrements. Le
premier comprend `limit=25` et les groupes nommés `name` (`limit`) et `value`
(`25`).

## Syntaxe RE2

Les motifs utilisent le moteur RE2 sûr de Go. Les groupes nommés utilisent
`(?P<name>...)`. Lookahead, lookbehind et références arrière dans le motif ne
sont pas pris en charge. Un motif invalide fait échouer le nœud sans conversion
ni interprétation implicite.
