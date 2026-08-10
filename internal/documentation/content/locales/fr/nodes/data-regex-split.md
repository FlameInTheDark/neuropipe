# Découpage regex

## Objectif

Découpe le **Texte** à chaque correspondance RE2 de Go. Définissez **Motif**
dans l’inspecteur ou connectez un fil Texte pour remplacer la valeur enregistrée
lors de l’exécution en cours.

## Sorties

- **Parties** est un `list[string]` exact.
- **Découpages** est le nombre entier exact de délimiteurs trouvés.
- **Correspondance trouvée** indique si le motif apparaît dans l’entrée.

Les parties vides initiales et finales sont conservées. Sans correspondance,
Parties contient uniquement le texte initial, Découpages vaut zéro et
Correspondance trouvée est false.

## Exemple

Utilisez le texte `one, two; three` avec le motif `[,;]\s*`. Parties vaut
`["one", "two", "three"]` et peut être envoyé directement vers **For Each**.

## Syntaxe RE2

Les motifs utilisent la syntaxe RE2 sûre de Go, et non PCRE. Lookahead,
lookbehind et références arrière dans le motif ne sont pas pris en charge. Les
expressions invalides échouent explicitement ; les valeurs non textuelles ne
sont jamais converties en texte.
