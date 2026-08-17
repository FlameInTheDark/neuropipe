# Extraction HTML

## Objectif
Extrait des valeurs exactes d'un document HTML avec des sélecteurs CSS, comme le nœud HTML de n8n. Chaque requête configurée crée sa propre broche de sortie typée.

## Configuration

Connectez un document HTML à l'entrée **HTML**, puis ajoutez des extractions dans l'inspecteur. Chaque extraction définit :

- le **nom de broche** du câble de sortie ;
- le **sélecteur CSS** des éléments à cibler, par exemple `h1.title` ou `ul li a` ;
- la **valeur de retour** : **Texte** (contenu textuel de l'élément), **HTML** (le balisage rendu de l'élément) ou **Attribut** (un attribut tel que `href` ; le champ du nom d'attribut apparaît quand il est choisi) ;
- **Renvoyer toutes les correspondances** : désactivé renvoie la première correspondance en Texte ; activé renvoie chaque correspondance dans une liste de Texte.

Les sorties n'utilisent que les types de données par défaut (Texte ou Liste) et se connectent donc directement à Formater le texte, Pour chaque, Joindre, Regex et tous les autres nœuds.

Un sélecteur sans correspondance n'est pas une erreur : la sortie est un texte vide, ou une liste vide en mode « toutes les correspondances ». Un sélecteur CSS invalide ou un nom de broche en double fait échouer la validation avant l'exécution du pipeline.

## Exemple
`Requête HTTP → Extraction HTML (sélecteur `a.product`, valeur de retour : Attribut `href`, toutes les correspondances) → Pour chaque`.

Combinez-le avec les commutateurs **Supprimer les scripts** et **Supprimer les styles** de la requête HTTP pour fournir un balisage propre à l'extracteur.
