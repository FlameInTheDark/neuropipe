# Décoder Base64

## Objectif

Décode explicitement une représentation Base64 sélectionnée **Texte** ou
**Octets**. Le sélecteur de sortie déclare les octets d’origine ou le texte
UTF-8.

## Exemple

`Lire un champ → Décoder Base64 → Écrire un fichier`.

Un Base64 malformé arrête le chemin actif en toute sécurité. Le choix Texte
pour des données binaires échoue aussi sûrement ; utilisez Octets.
