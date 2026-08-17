# Requête HTTP

## Objectif
Appelle un point de terminaison HTTP et met une réponse texte ou JSON à disposition du graphe.

## Configuration

- **URL** et **Méthode** définissent la cible et le verbe de la requête.
- **Body** est envoyé en JSON sauf si un en-tête `Content-Type` est configuré.
- **En-têtes de la requête** est un éditeur clé/valeur. Les noms d'en-tête
  répétés sont envoyés comme en-têtes séparés.
- Activez **Utiliser un User-Agent personnalisé** pour révéler le champ
  User-Agent. Il remplace tout `User-Agent` défini dans la liste d'en-têtes.
- **Supprimer les scripts** retire les éléments `script` et `noscript` du corps
  d'une réponse HTML ; **Supprimer les styles** retire les éléments `style` et
  les références `link rel="stylesheet"`. Les deux laissent le corps inchangé
  pour les réponses non HTML telles que JSON.

Le nœud ne s'exécute que lorsque son entrée d'exécution est pulsée. Les
en-têtes sont de la configuration, pas une broche de données : ils ne peuvent
pas déclencher une requête seuls.

## Résultat

L'objet résultat expose le code de statut HTTP, le corps de la réponse, les
en-têtes de réponse et le JSON analysé lorsque le corps est un JSON valide.
Toute réponse 4xx ou 5xx interrompt l'exécution en cours et apparaît dans le
journal d'exécution.

## Exemple
`Déclencheur de bouton → Requête HTTP → Créer un rapport` ; connectez Résultat à Lire un champ pour une propriété JSON.
