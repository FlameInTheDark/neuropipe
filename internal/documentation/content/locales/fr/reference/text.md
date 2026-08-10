# Nœuds Texte

Les nœuds Texte utilisent uniquement des valeurs Text exactes sans conversion
implicite. **Split** retourne `list[string]` en conservant les segments vides,
et **Join** accepte uniquement cette liste. **Contains**, **Starts With** et
**Ends With** sont sensibles à la casse. **Replace** remplace la première
occurrence, un nombre positif précis ou toutes les occurrences ; une recherche
vide échoue explicitement.

**Index Of** et **Substring** utilisent des décalages en points de code Unicode.
Une valeur absente donne l’index `-1` et une plage invalide échoue.
