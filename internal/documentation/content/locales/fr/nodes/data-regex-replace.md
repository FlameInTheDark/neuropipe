# Remplacement regex

## Objectif

Remplace chaque correspondance RE2 de Go dans le **Texte**. **Motif** et
**Remplacement** peuvent être configurés dans l’inspecteur ou fournis par des
fils Texte ; un fil connecté est prioritaire.

## Sorties

- **Texte** est le texte remplacé.
- **Remplacements** est le nombre entier exact de correspondances.
- **Modifié** indique si le texte de sortie diffère du texte d’entrée.

L’absence de correspondance n’est pas une erreur. Le texte initial est renvoyé
avec zéro remplacement et Modifié=false.

## Exemple

Avec le texte `Ada Lovelace`, le motif `(?P<first>\w+) (?P<last>\w+)` et le
remplacement `${last}, $1`, le texte de sortie est `Lovelace, Ada`.

## Syntaxe RE2 et remplacement

Les motifs utilisent la syntaxe RE2 sûre de Go. Le remplacement suit
`regexp.ReplaceAllString` : `$1` vise le premier groupe et `${name}` un groupe
nommé. Lookaround et références arrière dans le motif ne sont pas pris en
charge. Les valeurs ne sont jamais converties implicitement.
